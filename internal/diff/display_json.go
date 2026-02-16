package diff

type Report struct {
	Website     string `json:"website" yaml:"website"`
	Environment string `json:"environment" yaml:"environment"`
	Result      Result `json:"result" yaml:"result"`
}
