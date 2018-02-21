package protocol

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"crypto/cipher"
	"crypto/sha512"
	"hash"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/group/edwards25519"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
// If this name is used, then the suite used to verify signatures must be
// the default cothority.Suite.
const DefaultProtocolName = "CoSiProtoDefault"

// DefaultSubProtocolName the name of the default sub protocol, started by the
// main protocol.
const DefaultSubProtocolName = "SubCoSiProtoDefault"

type cosiSuite struct {
	cosi.Suite
	r cipher.Stream
}

func (m *cosiSuite) Hash() hash.Hash {
	return sha512.New()
}

func (m *cosiSuite) RandomStream() cipher.Stream {
	return m.r
}

// EdDSACompatibleCosiSuite is a custom suite made to be compatible with eddsa because
// cothority.Suite uses sha256 but EdDSA uses sha512.
var EdDSACompatibleCosiSuite = &cosiSuite{edwards25519.NewBlakeSHA256Ed25519(), random.New()}

// Announcement is the announcement message, the first message in the CoSi protocol
type Announcement struct {
	Msg     []byte
	Data    []byte
	Publics []kyber.Point
	Timeout time.Duration
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the sda framework.
type StructAnnouncement struct {
	*onet.TreeNode //sender
	Announcement
}

// Commitment is the cosi commitment message
type Commitment struct {
	CoSiCommitment kyber.Point
	Mask           []byte
}

// StructCommitment just contains Commitment and the data necessary to identify and
// process the message in the sda framework.
type StructCommitment struct {
	*onet.TreeNode
	Commitment
}

// Challenge is the cosi challenge message
type Challenge struct {
	CoSiChallenge kyber.Scalar
}

// StructChallenge just contains Challenge and the data necessary to identify and
// process the message in the sda framework.
type StructChallenge struct {
	*onet.TreeNode
	Challenge
}

// Response is the cosi response message
type Response struct {
	CoSiReponse kyber.Scalar
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the sda framework.
type StructResponse struct {
	*onet.TreeNode
	Response
}

// Stop is a message used to instruct a node to stop its protocol
type Stop struct{}

// StructStop is a wrapper around Stop for it to work with onet
type StructStop struct {
	*onet.TreeNode
	Stop
}
