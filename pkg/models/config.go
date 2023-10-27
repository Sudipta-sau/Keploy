package models

type Config struct {
	Record  Record    `json:"record" yaml:"record"`
	Test    Test      `json:"test" yaml:"test"`
}

type Record struct {
	Path string  `json:"path" yaml:"path"`
	Command string `json:"command" yaml:"command"`
	ContainerName string `json:"containerName" yaml:"containerName"`
	ProxyPort uint32 `json:"proxyport" yaml:"proxyport"`
	NetworkName string `json:"networkName" yaml:"networkName"`
	Delay uint64 `json:"delay" yaml:"delay"`
	PassThroughPorts []uint `json:"passThroughPorts" yaml:"passThroughPorts"`
}

type Test struct {
	Path string  `json:"path" yaml:"path"`
	ProxyPort uint32 `json:"proxyport" yaml:"proxyport"`
	Command string `json:"command" yaml:"command"`
	ContainerName string `json:"containerName" yaml:"containerName"`
	NetworkName string `json:"networkName" yaml:"networkName"`
	TestSets []string `json:"testSets" yaml:"testSets"`
	Noise string `json:"noise" yaml:"noise"`
	Delay uint64 `json:"delay" yaml:"delay"`
	ApiTimeout uint64 `json:"apiTimeout" yaml:"apiTimeout"`
	PassThroughPorts []uint `json:"passThroughPorts" yaml:"passThroughPorts"`
}