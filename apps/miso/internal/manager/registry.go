package manager

import "sort"

var driverRegistry = map[string]Manager{}

// register manager driver
func RegisterManager(name string, m Manager) {
	driverRegistry[name] = m
}

// return manager driver for given name
func GetManager(name string) (Manager, bool) {
	driver, ok := driverRegistry[name]
	return driver, ok
}

// return sorted list of all managers
func GetRegisteredManagers() []string {
	names := make([]string, 0, len(driverRegistry))
	for name := range driverRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
