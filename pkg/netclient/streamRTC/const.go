package streamRTC

const (
	maxPackSize  = 64 * 1024 * 1024
	_maxPackSize = 65535
)

const (
	FunHandshake  = "Handshake"
	FunHostOnline = "HostOnline"
	FunOffer      = "Offer"
	FunAnswer     = "Answer"
	FunCandidate  = "Candidate"
)
