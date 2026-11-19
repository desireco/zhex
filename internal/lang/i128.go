package lang

// Int's runtime width lives in the runtime package (zhexrt.Int) so
// zhex compile --target=native and the interpreter share one
// implementation; these aliases keep the interpreter's i128 spelling.

import "zhex/internal/zhexrt"

type i128 = zhexrt.Int

var (
	i128Max = zhexrt.Max
	i128Min = zhexrt.Min
)
