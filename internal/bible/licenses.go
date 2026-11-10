package bible

import (
	"errors"
	"fmt"
)

// Distribution says what may be done with a translation's text.
type Distribution int

const (
	DistributionUnknown Distribution = iota
	// DistributionPublicDomain: nobody owns the text; it may be bundled freely.
	DistributionPublicDomain
	// DistributionPermitted: the copyright holder has allowed this build.
	DistributionPermitted
	// DistributionRestricted: no permission has been recorded, so bundling is
	// refused until the publisher grants it in writing.
	DistributionRestricted
)

var (
	// ErrDistributionNotPermitted is returned for a translation the project may
	// not yet distribute.
	ErrDistributionNotPermitted = errors.New("this translation is under copyright and no permission to distribute it has been recorded")
	// ErrLicenseUnknown is returned when an unregistered translation is offered
	// for bundling without its license stated.
	ErrLicenseUnknown = errors.New("the license of this translation is not recorded; state it with -license and -attribution when importing")
)

// TranslationLicense carries the legal lines a translation travels with.
type TranslationLicense struct {
	Copyright   string
	License     string
	Attribution string
	Status      Distribution
}

// The registry states what is known about texts this project may one day carry.
// A restricted entry is a placeholder for permission that has to arrive in
// writing from its copyright holder, whose attribution wording is their own —
// so it is left empty here rather than guessed.
var knownLicenses = map[string]TranslationLicense{
	"kjv": {
		Copyright:   "Public domain",
		License:     "Public domain",
		Attribution: "King James Version (Public Domain)",
		Status:      DistributionPublicDomain,
	},
	"amp": {
		Copyright: "© The Lockman Foundation",
		License:   "Copyrighted",
		Status:    DistributionRestricted,
	},
	"nasb": {
		Copyright: "© The Lockman Foundation",
		License:   "Copyrighted",
		Status:    DistributionRestricted,
	},
	"niv": {
		Copyright: "© Biblica, Inc.",
		License:   "Copyrighted",
		Status:    DistributionRestricted,
	},
	"esv": {
		Copyright: "© Crossway (Good News Publishers)",
		License:   "Copyrighted",
		Status:    DistributionRestricted,
	},
	"nlt": {
		Copyright: "© Tyndale House Ministries",
		License:   "Copyrighted",
		Status:    DistributionRestricted,
	},
}

// LicenseFor returns the recorded license of a translation id, if one is known.
func LicenseFor(id string) (TranslationLicense, bool) {
	license, ok := knownLicenses[id]
	return license, ok
}

// CheckBundling refuses a translation this project may not distribute. It runs
// before a package is built, so copyrighted text cannot slip into a release by
// accident.
func CheckBundling(id string) error {
	license, ok := LicenseFor(id)
	if !ok {
		return ErrLicenseUnknown
	}
	if license.Status == DistributionRestricted {
		return fmt.Errorf("%w (%s)", ErrDistributionNotPermitted, license.Copyright)
	}
	return nil
}
