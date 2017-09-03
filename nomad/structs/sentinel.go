package structs

// SentinelObject converts the internal job into a Sentinel object.
// This format is used to preserve compatability should the internal representation change.
func (j *Job) SentinelObject() map[string]interface{} {
	out := map[string]interface{}{
		"id":            j.ID,
		"parent_id":     j.ParentID,
		"region":        j.Region,
		"name":          j.Name,
		"type":          j.Type,
		"priority":      j.Priority,
		"all_at_once":   j.AllAtOnce,
		"datacenters":   j.Datacenters,
		"constraints":   nil,
		"task_groups":   nil,
		"periodic":      j.Periodic.SentinelObject(),
		"parameterized": j.ParameterizedJob.SentinelObject(),
		"payload":       j.Payload,
		"meta":          j.Meta,
	}

	// Convert the constraints
	if len(j.Constraints) > 0 {
		list := make([]map[string]interface{}, len(j.Constraints))
		for idx, c := range j.Constraints {
			list[idx] = c.SentinelObject()
		}
		out["constraints"] = list
	}

	// Convert the task groups
	if len(j.TaskGroups) > 0 {
		list := make([]map[string]interface{}, len(j.TaskGroups))
		for idx, tg := range j.TaskGroups {
			list[idx] = tg.SentinelObject()
		}
		out["task_groups"] = list
	}
	return out
}

// SentinelObject converts the internal Constraint into a Sentinel form.
func (c *Constraint) SentinelObject() map[string]interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"operand":      c.Operand,
		"left_target":  c.LTarget,
		"right_target": c.RTarget,
		"string":       c.String(),
	}
}

// SentinelObject converts the PeriodicConfig into a Sentinel form.
func (p *PeriodicConfig) SentinelObject() map[string]interface{} {
	if p == nil || !p.Enabled {
		return nil
	}
	return map[string]interface{}{
		"spec":             p.Spec,
		"spec_type":        p.SpecType,
		"prohibit_overlap": p.ProhibitOverlap,
		"timezone":         p.TimeZone,
	}
}

// SentinelObject converts the ParameterizedJobConfig into a Sentinel form.
func (p *ParameterizedJobConfig) SentinelObject() map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"payload_type":  p.Payload,
		"meta_required": p.MetaRequired,
		"meta_optional": p.MetaOptional,
	}
}

// SentinelObject converts the TaskGroup into a Sentinel form
func (tg *TaskGroup) SentinelObject() map[string]interface{} {
	if tg == nil {
		return nil
	}
	out := map[string]interface{}{
		"name":           tg.Name,
		"count":          tg.Count,
		"update":         tg.Update.SentinelObject(),
		"constraints":    nil,
		"restart_policy": tg.RestartPolicy.SentinelObject(),
		"tasks":          nil,
		"ephemeral_disk": tg.EphemeralDisk.SentinelObject(),
		"meta":           tg.Meta,
	}

	// Convert the constraints
	if len(tg.Constraints) > 0 {
		list := make([]map[string]interface{}, len(tg.Constraints))
		for idx, c := range tg.Constraints {
			list[idx] = c.SentinelObject()
		}
		out["constraints"] = list
	}

	// Convert the tasks
	if len(tg.Tasks) > 0 {
		list := make([]map[string]interface{}, len(tg.Tasks))
		for idx, task := range tg.Tasks {
			list[idx] = task.SentinelObject()
		}
		out["tasks"] = list
	}
	return out
}

// SentinelObject converts the UpdateStrategy into a Sentinel form
func (u *UpdateStrategy) SentinelObject() map[string]interface{} {
	if u == nil {
		return nil
	}
	out := map[string]interface{}{
		"stagger":          u.Stagger,
		"max_parallel":     u.MaxParallel,
		"health_check":     u.HealthCheck,
		"min_healthy_time": u.MinHealthyTime,
		"healthy_deadline": u.HealthyDeadline,
		"auto_revert":      u.AutoRevert,
		"canary":           u.Canary,
	}
	return out
}

// SentinelObject converts the RestartPolicy into a Sentinel form
func (r *RestartPolicy) SentinelObject() map[string]interface{} {
	if r == nil {
		return nil
	}
	out := map[string]interface{}{
		"attempts": r.Attempts,
		"interval": r.Interval,
		"delay":    r.Delay,
		"mode":     r.Mode,
	}
	return out
}

// SentinelObject converts the EphemeralDisk into a Sentinel form
func (e *EphemeralDisk) SentinelObject() map[string]interface{} {
	if e == nil {
		return nil
	}
	out := map[string]interface{}{
		"sticky":  e.Sticky,
		"size_mb": e.SizeMB,
		"migrate": e.Migrate,
	}
	return out
}

// SentinelObject converts the Task into a Sentinel form
func (t *Task) SentinelObject() map[string]interface{} {
	if t == nil {
		return nil
	}
	out := map[string]interface{}{
		"name":             t.Name,
		"driver":           t.Driver,
		"config":           t.Config,
		"user":             t.User,
		"env":              t.Env,
		"services":         nil,
		"vault":            t.Vault.SentinelObject(),
		"templates":        nil,
		"constraints":      nil,
		"resources":        t.Resources.SentinelObject(),
		"dispatch_payload": t.DispatchPayload.SentinelObject(),
		"meta":             t.Meta,
		"kill_timeout":     t.KillTimeout,
		"log_config":       t.LogConfig.SentinelObject(),
		"artifacts":        nil,
		"leader":           t.Leader,
	}

	// Convert the services
	if len(t.Services) > 0 {
		list := make([]map[string]interface{}, len(t.Services))
		for idx, srv := range t.Services {
			list[idx] = srv.SentinelObject()
		}
		out["services"] = list
	}

	// Convert the templates
	if len(t.Templates) > 0 {
		list := make([]map[string]interface{}, len(t.Templates))
		for idx, tmp := range t.Templates {
			list[idx] = tmp.SentinelObject()
		}
		out["templates"] = list
	}

	// Convert the constraints
	if len(t.Constraints) > 0 {
		list := make([]map[string]interface{}, len(t.Constraints))
		for idx, c := range t.Constraints {
			list[idx] = c.SentinelObject()
		}
		out["constraints"] = list
	}

	// Convert the artifacts
	if len(t.Artifacts) > 0 {
		list := make([]map[string]interface{}, len(t.Artifacts))
		for idx, a := range t.Artifacts {
			list[idx] = a.SentinelObject()
		}
		out["artifacts"] = list
	}
	return out
}

// SentinelObject converts the Service into a Sentinel form
func (s *Service) SentinelObject() map[string]interface{} {
	if s == nil {
		return nil
	}
	out := map[string]interface{}{
		"name":         s.Name,
		"port_label":   s.PortLabel,
		"address_mode": s.AddressMode,
		"tags":         s.Tags,
		"checks":       nil,
	}

	// Convert the checks
	if len(s.Checks) > 0 {
		list := make([]map[string]interface{}, len(s.Checks))
		for idx, c := range s.Checks {
			list[idx] = c.SentinelObject()
		}
		out["checks"] = list
	}
	return out
}

// SentinelObject converts the ServiceCheck into a Sentinel form
func (c *ServiceCheck) SentinelObject() map[string]interface{} {
	if c == nil {
		return nil
	}
	out := map[string]interface{}{
		"name":            c.Name,
		"type":            c.Type,
		"command":         c.Command,
		"args":            c.Args,
		"path":            c.Path,
		"protocol":        c.Protocol,
		"port_label":      c.PortLabel,
		"interval":        c.Interval,
		"timeout":         c.Timeout,
		"initial_status":  c.InitialStatus,
		"tls_skip_verify": c.TLSSkipVerify,
	}
	return out
}

// SentinelObject converts the Vault into a Sentinel form
func (v *Vault) SentinelObject() map[string]interface{} {
	if v == nil {
		return nil
	}
	out := map[string]interface{}{
		"policies":      v.Policies,
		"env":           v.Env,
		"change_mode":   v.ChangeMode,
		"change_signal": v.ChangeSignal,
	}
	return out
}

// SentinelObject converts the Template into a Sentinel form
func (t *Template) SentinelObject() map[string]interface{} {
	if t == nil {
		return nil
	}
	out := map[string]interface{}{
		"source_path":       t.SourcePath,
		"destination_path":  t.DestPath,
		"embedded_tempalte": t.EmbeddedTmpl,
		"change_mode":       t.ChangeMode,
		"change_signal":     t.ChangeSignal,
		"splay":             t.Splay,
		"permissions":       t.Perms,
		"left_delimiter":    t.LeftDelim,
		"right_delimiter":   t.RightDelim,
		"env_vars":          t.Envvars,
	}
	return out
}

// SentinelObject converts the Resources into a Sentinel form
func (r *Resources) SentinelObject() map[string]interface{} {
	if r == nil {
		return nil
	}
	out := map[string]interface{}{
		"cpu":       r.CPU,
		"memory_mb": r.MemoryMB,
		"disk_mb":   r.DiskMB,
		"iops":      r.IOPS,
		"networks":  nil,
	}

	// Convert the networks
	if len(r.Networks) > 0 {
		list := make([]map[string]interface{}, len(r.Networks))
		for idx, n := range r.Networks {
			list[idx] = n.SentinelObject()
		}
		out["networks"] = list
	}
	return out
}

// SentinelObject converts the NetworkResource into a Sentinel form
func (r *NetworkResource) SentinelObject() map[string]interface{} {
	if r == nil {
		return nil
	}
	out := map[string]interface{}{
		"device":         r.Device,
		"cidr":           r.CIDR,
		"ip":             r.IP,
		"mbits":          r.MBits,
		"reserved_ports": nil,
		"dynamic_ports":  nil,
	}

	// Convert the ports
	if len(r.ReservedPorts) > 0 {
		list := make([]map[string]interface{}, len(r.ReservedPorts))
		for idx, p := range r.ReservedPorts {
			list[idx] = p.SentinelObject()
		}
		out["reserved_ports"] = list
	}
	if len(r.DynamicPorts) > 0 {
		list := make([]map[string]interface{}, len(r.DynamicPorts))
		for idx, p := range r.DynamicPorts {
			list[idx] = p.SentinelObject()
		}
		out["dynamic_ports"] = list
	}
	return out
}

// SentinelObject converts a Port into a Sentinel form
func (p Port) SentinelObject() map[string]interface{} {
	out := map[string]interface{}{
		"label": p.Label,
		"value": p.Value,
	}
	return out
}

// SentinelObject converts the DispatchPayloadConfig into a Sentinel form
func (d *DispatchPayloadConfig) SentinelObject() map[string]interface{} {
	if d == nil {
		return nil
	}
	out := map[string]interface{}{
		"file": d.File,
	}
	return out
}

// SentinelObject converts the LogConfig into a Sentinel form
func (l *LogConfig) SentinelObject() map[string]interface{} {
	if l == nil {
		return nil
	}
	out := map[string]interface{}{
		"max_files":       l.MaxFiles,
		"max_filesize_mb": l.MaxFileSizeMB,
	}
	return out
}

// SentinelObject converts the TaskArtifact into a Sentinel form
func (a *TaskArtifact) SentinelObject() map[string]interface{} {
	if a == nil {
		return nil
	}
	out := map[string]interface{}{
		"source":               a.GetterSource,
		"options":              a.GetterOptions,
		"getter_mode":          a.GetterMode,
		"relative_destination": a.RelativeDest,
	}
	return out
}
