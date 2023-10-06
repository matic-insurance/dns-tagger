package provider

type Provider interface {
	Whoami() string
}

type BaseProvider struct {
}
