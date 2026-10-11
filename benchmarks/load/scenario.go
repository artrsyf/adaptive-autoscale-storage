package main

// workloadRate keeps controller warm-up at its neutral steady stage so
// pre-measurement traffic can flush seed samples without producing a recommendation.
func workloadRate(scenario, phase string, baseRate, fraction float64) (float64, string) {
	if phase == "measure" {
		return scenarioRate(scenario, baseRate, fraction)
	}
	if phase == "warmup" && scenario == "controller" {
		rate, _ := scenarioRate(scenario, baseRate, .2)
		return rate, "inactive"
	}
	return baseRate, "inactive"
}

// scenarioRate returns the offered rate and bounded stage label for one measurement position.
func scenarioRate(scenario string, baseRate, fraction float64) (float64, string) {
	switch scenario {
	case "ramp":
		return baseRate * (.2 + .8*fraction), "ramp"
	case "burst":
		if fraction >= .4 && fraction < .6 {
			return baseRate * 3, "burst"
		}
		return baseRate, "steady"
	case "controller":
		switch {
		case fraction < .15:
			return baseRate * .15, "low"
		case fraction < .3:
			return baseRate * .45, "steady"
		case fraction < .5:
			return baseRate * (.45 + ((fraction-.3)/.2)*.75), "ramp"
		case fraction < .56:
			return baseRate * 2.5, "short_burst"
		case fraction < .75:
			return baseRate * 1.2, "high"
		default:
			return baseRate * .15, "recovery"
		}
	default:
		return baseRate, "steady"
	}
}
