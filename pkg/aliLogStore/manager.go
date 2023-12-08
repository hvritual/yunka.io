package aliLogStore

import (
	"errors"
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
	"io"
	"yunka.io/pkg/resource"
)

type (
	Config struct {
		Usage           string
		LogStore        string
		ProjectName     string
		Endpoint        string
		AccessKeyID     string
		AccessKeySecret string
		SecurityToken   string
		Topic           string
		Source          string
	}

	logStore struct {
		*sls.LogStore
	}
	project struct {
		*sls.LogProject
	}
	provider struct {
		sls.CredentialsProvider
	}

	Manager struct {
		configs       map[string]*Config
		resources     *resource.Manager
		writer        io.Writer
		defaultConfig *Config
	}
)

func NewManager(cnfs []*Config, w io.Writer) *Manager {
	if cnfs == nil || len(cnfs) == 0 {
		return nil
	}
	var m = &Manager{
		configs:   map[string]*Config{},
		resources: resource.NewManager(),
		writer:    w,
	}

	for i := 0; i < len(cnfs); i++ {
		m.configs[cnfs[i].Usage] = cnfs[i]
	}
	m.defaultConfig = cnfs[0]

	return m
}

func (c *logStore) Close() error { return nil }
func (c *project) Close() error  { return nil }
func (c *provider) Close() error { return nil }

func (m *Manager) Default() *sls.LogStore {
	ls, err := m.create(m.defaultConfig)
	if err != nil {
		panic(err)
	}
	return ls.LogStore
}

func (m *Manager) Use(usage string) (*sls.LogStore, error) {
	cnf, ok := m.configs[usage]
	if !ok {
		return nil, errors.New("nil config")
	}
	ls, err := m.create(cnf)
	if err != nil {
		return nil, err
	}
	return ls.LogStore, nil
}

func (m *Manager) MustUse(usage string) *sls.LogStore {
	ls, err := m.Use(usage)
	if err != nil {
		panic(err)
	}
	return ls
}

func (m *Manager) AddConfig(cfg *Config) {
	if _, ok := m.configs[cfg.Usage]; !ok {
		m.configs[cfg.Usage] = cfg
	}
}
func (m *Manager) SetWrite(w io.Writer) {
	m.writer = w
}

func (m *Manager) create(cnf *Config) (*logStore, error) {
	ls, err := m.createLogStore(cnf)
	if err != nil {
		return nil, err
	}
	if m.writer != nil {
		sls.Logger = log.NewLogfmtLogger(m.writer)
	}

	return ls.(*logStore), nil
}

func (m *Manager) createLogStore(cnf *Config) (io.Closer, error) {
	return m.resources.Get(fmt.Sprintf("log_store.%s.%s.%s", cnf.AccessKeyID, cnf.ProjectName, cnf.LogStore),
		func() (io.Closer, error) {
			pro, err := m.createProject(cnf)
			if err != nil {
				return nil, err
			}

			ls, err := m.createSlsLogStore(cnf, pro.(*project).LogProject)
			if err != nil {
				return nil, err
			}

			return &logStore{ls}, nil
		})
}

func (m *Manager) createSlsLogStore(cnf *Config, project *sls.LogProject) (*sls.LogStore, error) {
	return sls.NewLogStore(cnf.LogStore, project)
}

func (m *Manager) createProject(cnf *Config) (io.Closer, error) {
	return m.resources.Get(fmt.Sprintf("project.%s.%s", cnf.AccessKeyID, cnf.ProjectName), func() (io.Closer, error) {
		pro, _ := m.createProvider(cnf)

		p1, err := m.createSlsProject(cnf, pro.(*provider))
		if err != nil {
			return nil, err
		}
		return &project{p1}, nil

	})
}

func (m *Manager) createSlsProject(cnf *Config, provider sls.CredentialsProvider) (*sls.LogProject, error) {
	return sls.NewLogProjectV2(cnf.ProjectName, cnf.Endpoint, provider)
}

func (m *Manager) createProvider(cnf *Config) (io.Closer, error) {
	return m.resources.Get(fmt.Sprintf("provider.%s", cnf.AccessKeyID), func() (io.Closer, error) {
		return &provider{m.createSlsProvider(cnf)}, nil
	})
}

func (m *Manager) createSlsProvider(cnf *Config) *sls.StaticCredentialsProvider {
	return sls.NewStaticCredentialsProvider(cnf.AccessKeyID, cnf.AccessKeySecret, cnf.SecurityToken)
}
