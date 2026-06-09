package data

type Class int

const (
	Normal Class = iota
	DoS
	Probe
	R2L
	U2R
)

const NumClasses = 5

var classNames = [NumClasses]string{"Normal", "DoS", "Probe", "R2L", "U2R"}

func (c Class) String() string {
	if c < 0 || int(c) >= NumClasses {
		return "Unknown"
	}
	return classNames[c]
}

var attackToClass = map[string]Class{
	"normal": Normal,

	"back": DoS, "land": DoS, "neptune": DoS, "pod": DoS, "smurf": DoS,
	"teardrop": DoS, "apache2": DoS, "udpstorm": DoS, "processtable": DoS,
	"worm": DoS, "mailbomb": DoS,

	"satan": Probe, "ipsweep": Probe, "nmap": Probe, "portsweep": Probe,
	"mscan": Probe, "saint": Probe,

	"guess_passwd": R2L, "ftp_write": R2L, "imap": R2L, "phf": R2L,
	"multihop": R2L, "warezmaster": R2L, "warezclient": R2L, "spy": R2L,
	"xlock": R2L, "xsnoop": R2L, "snmpguess": R2L, "snmpgetattack": R2L,
	"httptunnel": R2L, "sendmail": R2L, "named": R2L,

	"buffer_overflow": U2R, "loadmodule": U2R, "rootkit": U2R, "perl": U2R,
	"sqlattack": U2R, "xterm": U2R, "ps": U2R,
}

func ClassOf(attack string) (Class, bool) {
	c, ok := attackToClass[attack]
	return c, ok
}
