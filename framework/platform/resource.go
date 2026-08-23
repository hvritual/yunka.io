package platform

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"gorm.io/gorm"
)

// DatabaseResource is one App-owned named database capability. The database
// handle is exposed to typed module wiring while lifecycle callbacks stay
// inside the platform provider.
type DatabaseResource struct {
	Database     *gorm.DB
	StartFunc    func(context.Context) error
	HealthFunc   func(context.Context) error
	ShutdownFunc func(context.Context) error
}

// BorrowedDatabase exposes a GORM handle without taking close ownership. It is
// intended for embedding or tests where a wider owner already controls the
// connection pool.
func BorrowedDatabase(database *gorm.DB) DatabaseResource {
	return DatabaseResource{
		Database:   database,
		HealthFunc: gormHealth(database),
	}
}

// ManagedDatabase exposes a GORM handle and owns the underlying sql.DB pool.
// Shutdown closes the pool exactly once through Provider.Shutdown.
func ManagedDatabase(database *gorm.DB) DatabaseResource {
	health := gormHealth(database)
	return DatabaseResource{
		Database:   database,
		StartFunc:  health,
		HealthFunc: health,
		ShutdownFunc: func(context.Context) error {
			if database == nil {
				return nil
			}
			sqlDB, err := database.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
	}
}

func gormHealth(database *gorm.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		if database == nil {
			return errors.New("platform: database is nil")
		}
		sqlDB, err := database.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(normalizeContext(ctx))
	}
}

// RPCResource is one App-owned named typed gRPC connection capability.
type RPCResource struct {
	Connection   grpc.ClientConnInterface
	StartFunc    func(context.Context) error
	HealthFunc   func(context.Context) error
	ShutdownFunc func(context.Context) error
}

// BorrowedRPC exposes an existing typed connection without taking lifecycle
// ownership.
func BorrowedRPC(connection grpc.ClientConnInterface) RPCResource {
	return RPCResource{Connection: connection}
}

// ManagedClientConn owns a grpc.ClientConn. Start establishes readiness within
// the App start budget, Health requires READY, and Shutdown closes the
// connection.
func ManagedClientConn(connection *grpc.ClientConn) RPCResource {
	return RPCResource{
		Connection: connection,
		StartFunc: func(ctx context.Context) error {
			if connection == nil {
				return errors.New("platform: gRPC connection is nil")
			}
			ctx = normalizeContext(ctx)
			connection.Connect()
			for {
				state := connection.GetState()
				switch state {
				case connectivity.Ready:
					return nil
				case connectivity.Shutdown:
					return errors.New("platform: gRPC connection is shutdown")
				}
				if !connection.WaitForStateChange(ctx, state) {
					if err := ctx.Err(); err != nil {
						return err
					}
					return fmt.Errorf("platform: gRPC connection did not leave state %s", state)
				}
			}
		},
		HealthFunc: func(context.Context) error {
			if connection == nil {
				return errors.New("platform: gRPC connection is nil")
			}
			if state := connection.GetState(); state != connectivity.Ready {
				return fmt.Errorf("platform: gRPC connection state is %s", state)
			}
			return nil
		},
		ShutdownFunc: func(context.Context) error {
			if connection == nil {
				return nil
			}
			return connection.Close()
		},
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
