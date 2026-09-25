module example.com/bcg-engine-public-v2

go 1.22.0

require github.com/shmatovv/bcg-engine/v2 v2.0.0

// This makes the example self-contained in a source checkout. Consumers
// remove the replace line and use the published v2 module version.
replace github.com/shmatovv/bcg-engine/v2 => ../..
