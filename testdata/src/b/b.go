// Package b exercises write facts imported across packages.
package b

type Counter struct {
	N   int
	Log []string
}

func (c *Counter) Inc() { // want Inc:"recv=direct"
	c.N++
}

func (c *Counter) Get() int {
	return c.N
}

func (c *Counter) Record(s string) { // want Record:"recv=direct"
	c.Log = append(c.Log, s)
}

func (c *Counter) Touch() { // want Touch:"recv=shared"
	c.Log[0] = "touched"
}

func (c *Counter) Next() int { // want Next:"recv=direct"
	c.N++

	return c.N
}

func (c *Counter) Validate() error { // want Validate:"recv=direct"
	if c.N < 0 {
		c.N = 0
	}

	return nil
}

func Reset(c *Counter) { // want Reset:"recv=none p0=direct"
	c.N = 0
}

func Inspect(c *Counter) int {
	return c.N
}

var saved *Counter

func Store(c *Counter) { // want Store:"recv=none p0=shared"
	saved = c
}

func Update(c *Counter, n int) error { // want Update:"recv=none p0=direct p1=none"
	c.N = n

	return nil
}
