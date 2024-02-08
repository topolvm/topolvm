package lvmd

type LvcreateOptionClass struct {
	// Name for the lvcreate-option-class name
	Name string `json:"name"`
	// Options are extra arguments to pass to lvcreate
	Options []string `json:"options"`
}

type LvcreateOptionClassManager struct {
	LvcreateOptionClassByName map[string]*LvcreateOptionClass
}

// NewLvcreateOptionClassManager creates a new LvcreateOptionClassManager
func NewLvcreateOptionClassManager(LvcreateOptionClasses []*LvcreateOptionClass) *LvcreateOptionClassManager {
	cm := LvcreateOptionClassManager{}
	cm.LvcreateOptionClassByName = make(map[string]*LvcreateOptionClass)
	for _, c := range LvcreateOptionClasses {
		cm.LvcreateOptionClassByName[c.Name] = c
	}
	return &cm
}

// LvcreateOptionClassClass returns the lvcreate-option-class by its name
func (m LvcreateOptionClassManager) LvcreateOptionClass(name string) *LvcreateOptionClass {
	return m.LvcreateOptionClassByName[name]
}
