container {
	secrets {
		all = false
	}

       dependencies    = false
       alpine_security = false
}

binary {
	go_modules = false
	osv        = false
	nvd        = false

	secrets {
		all = true
	}
}
