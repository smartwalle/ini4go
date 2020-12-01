package ini4go

import (
	"fmt"
	"testing"
)

type TestConfig struct {
	Default Default
	HTTP    HTTP
	TCP     TCP `ini:"tcp"`
}

type Default struct {
	K1 int      `ini:"k1"`
	K2 int32    `ini:"k2"`
	K3 int64    `ini:"k3"`
	K4 uint32   `ini:"k4"`
	K5 string   `ini:"k5"`
	K6 bool     `ini:"k6"`
	K7 []string `ini:"k7"`
	K8 []int    `ini:"k8"`
	K9 string   `ini:"k9"`
}

type HTTP struct {
	IP   string `ini:"ip"`
	Port string `ini:"port"`
}

func (this *HTTP) Address() string {
	return fmt.Sprintf("%s:%s", this.IP, this.Port)
}

type TCP struct {
	IP   string `ini:"ip"`
	Port string `ini:"port"`
}

func (this *TCP) Address() string {
	return fmt.Sprintf("%s:%s", this.IP, this.Port)
}

func TestUnmarshal(t *testing.T) {
	var ini = New(false)
	if err := ini.LoadFiles("./marshal.conf"); err != nil {
		t.Fatal(err)
	}

	var tc *TestConfig
	if err := ini.Unmarshal(&tc); err != nil {
		t.Fatal(err)
	}

	t.Log(tc.Default.K1)
	t.Log(tc.Default.K2)
	t.Log(tc.Default.K3)
	t.Log(tc.Default.K4)
	t.Log(tc.Default.K5)
	t.Log(tc.Default.K6)
	t.Log(tc.Default.K7)
	t.Log(tc.Default.K8)
	t.Log(tc.Default.K9)
	t.Log(tc.HTTP.Address())
	t.Log(tc.TCP.Address())

	//t.Log(tc.Default.DK1, tc.Default.Dk2)
}
