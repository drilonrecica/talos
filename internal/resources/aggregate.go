// SPDX-License-Identifier: AGPL-3.0-only
package resources

type Component struct {
	CPU, Memory, RX, TX, Read, Write *float64
	PIDs                             *uint64
}
type Aggregate struct {
	CPU, Memory, RX, TX, Read, Write *float64
	PIDs                             *uint64
	Active                           int
}

func AggregateComponents(in []Component) Aggregate {
	var out Aggregate
	for _, c := range in {
		out.Active++
		out.CPU = add(out.CPU, c.CPU)
		out.Memory = add(out.Memory, c.Memory)
		out.RX = add(out.RX, c.RX)
		out.TX = add(out.TX, c.TX)
		out.Read = add(out.Read, c.Read)
		out.Write = add(out.Write, c.Write)
		if c.PIDs != nil {
			if out.PIDs == nil {
				v := uint64(0)
				out.PIDs = &v
			}
			*out.PIDs += *c.PIDs
		}
	}
	return out
}
func add(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a + *b
	return &v
}
