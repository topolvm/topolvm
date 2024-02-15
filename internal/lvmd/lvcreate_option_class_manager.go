package lvmd

import (
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
)

type LvcreateOptionClassManager struct {
	LvcreateOptionClassByName map[string]*lvmdTypes.LvcreateOptionClass
}

// NewLvcreateOptionClassManager creates a new LvcreateOptionClassManager
func NewLvcreateOptionClassManager(LvcreateOptionClasses []*lvmdTypes.LvcreateOptionClass) *LvcreateOptionClassManager {
	cm := LvcreateOptionClassManager{}
	cm.LvcreateOptionClassByName = make(map[string]*lvmdTypes.LvcreateOptionClass)
	for _, c := range LvcreateOptionClasses {
		cm.LvcreateOptionClassByName[c.Name] = c
	}
	return &cm
}

// LvcreateOptionClassClass returns the lvcreate-option-class by its name
func (m LvcreateOptionClassManager) LvcreateOptionClass(name string) *lvmdTypes.LvcreateOptionClass {
	return m.LvcreateOptionClassByName[name]
}
