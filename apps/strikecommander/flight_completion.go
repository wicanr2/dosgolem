package strikecommander

// completionBaseFailure is one bounded failed candidate, separate from the
// established Flight base and its original revalidation clock (013c §18).
type completionBaseFailure struct {
	valid       bool
	base        uint16
	lastAttempt uint64
}

// discoverCUCompletion only learns from a real CU entry. It never accepts a CU:
// Observe still sends the current entry through the original cuTracker.
func (o *FlightObserver) discoverCUCompletion(v FlightView) {
	if !o.discoverCompletionCU || (v.IP() != ipCUDiff && v.IP() != ipCUPlain) || v.CX()>>8 != 0 {
		return
	}
	base, steps := v.CS()-segFJemm, v.Steps()
	same := o.baseKnown && o.base == base
	if same {
		if !o.baseOK && steps-o.lastVerify < flightReverifySteps {
			return
		}
	} else if f := o.failedCompletionBase; f.valid && f.base == base && steps-f.lastAttempt < flightReverifySteps {
		return
	}

	// The wrapper also exists in MENU. It is only a cheap first rejection;
	// successful discovery always checks the entire STRIKE identity as well.
	why := "JEMM"
	if match(v, v.CS(), 0x0090, sigJemm, 0) {
		why = o.checkBase(v, base)
	}
	if why != "" {
		if same {
			o.lastVerify = steps
			if o.baseOK {
				o.baseOK = false
				o.cu.SetBase(o.base, false)
				o.reset()
				o.note(steps, "base", "CU base %04X 停用：%s 簽章不符", base, why)
			}
		} else {
			o.failedCompletionBase = completionBaseFailure{valid: true, base: base, lastAttempt: steps}
			o.note(steps, "base", "CU 候選 %04X 未啟用：%s 簽章不符", base, why)
		}
		return
	}
	o.failedCompletionBase = completionBaseFailure{}
	o.lastVerify = steps
	if same && o.baseOK {
		return
	}
	o.reset()
	o.base, o.baseKnown, o.baseOK = base, true, true
	o.cu.SetBase(base, true)
	o.note(steps, "base", "CU base %04X 啟用", base)
}
