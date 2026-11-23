package main

import (
	"encoding/json"
	"flag"
	"fmt"
)

func init() {
	register(command{
		Path: []string{"containers", "limits"}, Summary: "Show or set how much CPU, memory and how many processes a container may use (changes at once, no restart)",
		Usage: "containers limits <container> [--cpus N] [--memory MB] [--pids N] [--none]", AdminOnly: true,
		Examples: []string{
			"docklite containers limits example.com",
			"docklite containers limits example.com --cpus 0.5 --memory 512 --pids 200",
			"docklite containers limits example.com --none",
		},
		Run: func(a *app, args []string) error {
			var cpus float64
			var mem, pids int64
			var none bool
			pos, err := a.flags("containers limits", args, func(fs *flag.FlagSet) {
				fs.Float64Var(&cpus, "cpus", 0, "")
				fs.Int64Var(&mem, "memory", 0, "")
				fs.Int64Var(&pids, "pids", 0, "")
				fs.BoolVar(&none, "none", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite containers limits <container> [--cpus N] [--memory MB] [--pids N] [--none]")
			}
			c, err := a.resolveContainer(pos[0])
			if err != nil {
				return err
			}
			path := "/api/containers/" + c.ID + "/limits"
			setting := none || cpus != 0 || mem != 0 || pids != 0
			var data []byte
			if setting {
				// Changing one number keeps the others as they are, unless --none.
				l := map[string]any{"cpus": cpus, "memoryMB": mem, "pids": pids}
				if !none {
					if cur, gerr := a.get(path); gerr == nil {
						var cr struct {
							Limits struct {
								CPUs     float64 `json:"cpus"`
								MemoryMB int64   `json:"memoryMB"`
								Pids     int64   `json:"pids"`
							} `json:"limits"`
						}
						_ = json.Unmarshal(cur, &cr)
						if cpus == 0 {
							l["cpus"] = cr.Limits.CPUs
						}
						if mem == 0 {
							l["memoryMB"] = cr.Limits.MemoryMB
						}
						if pids == 0 {
							l["pids"] = cr.Limits.Pids
						}
					}
				}
				data, err = a.post(path, l)
			} else {
				data, err = a.get(path)
			}
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() {
				var r struct {
					Limits struct {
						CPUs     float64 `json:"cpus"`
						MemoryMB int64   `json:"memoryMB"`
						Pids     int64   `json:"pids"`
					} `json:"limits"`
					Host struct {
						CPUs     int   `json:"cpus"`
						MemoryMB int64 `json:"memoryMB"`
					} `json:"host"`
				}
				_ = json.Unmarshal(data, &r)
				show := func(set bool, v string, of string) string {
					if !set {
						return "no limit"
					}
					return v + of
				}
				a.say("%s: CPU %s, memory %s, processes %s", c.Name,
					show(r.Limits.CPUs > 0, fmt.Sprintf("%g cores", r.Limits.CPUs), fmt.Sprintf(" (of %d)", r.Host.CPUs)),
					show(r.Limits.MemoryMB > 0, fmt.Sprintf("%d MB", r.Limits.MemoryMB), fmt.Sprintf(" (of %d MB)", r.Host.MemoryMB)),
					show(r.Limits.Pids > 0, fmt.Sprintf("%d", r.Limits.Pids), ""))
			})
			return nil
		},
	})
}
