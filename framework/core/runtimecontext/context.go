package runtimecontext

import "context"

// Metadata describes the current operation without depending on a concrete
// transport. Transport adapters derive a child context with their own metadata.
type Metadata struct {
	Transport  string
	Protocol   string
	Operation  string
	Route      string
	Service    string
	Module     string
	Method     string
	RequestID  string
	Attributes map[string]string
}

func (m Metadata) Clone() Metadata {
	clone := m
	if m.Attributes != nil {
		clone.Attributes = make(map[string]string, len(m.Attributes))
		for key, value := range m.Attributes {
			clone.Attributes[key] = value
		}
	}
	return clone
}

type metadataContextKey struct{}
type traceIDContextKey struct{}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, metadataContextKey{}, metadata.Clone())
}

func MetadataFrom(ctx context.Context) (Metadata, bool) {
	if ctx == nil {
		return Metadata{}, false
	}
	metadata, ok := ctx.Value(metadataContextKey{}).(Metadata)
	if !ok {
		return Metadata{}, false
	}
	return metadata.Clone(), true
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(traceIDContextKey{}).(string)
	return traceID
}
