// Package demo holds the switch for demo mode (DOCKLITE_DEMO=1): a separate DockLite
// instance full of fake data, safe for screenshots, testing and a public demo. In demo
// mode nothing touches the host's nginx or certificates, and only containers tagged as
// demo containers are shown (and real instances never show them).
package demo

import "os"

// On is true when the agent was started with DOCKLITE_DEMO=1.
var On = os.Getenv("DOCKLITE_DEMO") == "1"

// Label marks containers that belong to the demo instance.
const Label = "docklite.demo"
