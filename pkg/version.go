package pkg

var (
	// MajorVersion is the API's major version.
    MajorVersion ="2"

	// MinorVersion is the API's minor version.
    MinorVersion ="7"

	// BuildNumber is the API's build number.
    BuildNumber ="2"

	// CommitNumber is the API's last git commit value.
	CommitNumber = ""
)

// GetVersion returns a string representation of the API version. The format is Major.Minor.Build.
func GetVersion() string {
	return (MajorVersion + "." + MinorVersion + "." + BuildNumber)
}
