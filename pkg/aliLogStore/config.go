package aliLogStore

// Config ali Log store config
type Config struct {
	Endpoint        string
	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
	Project         string
	LogStore        string
	Topic           string
	Source          string
}
