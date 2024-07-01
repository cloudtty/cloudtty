package remotesigner

import (
	"context"
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"
)

type SigAlgo string

const (
	// rfc8017

	SigAlgoRsaPssSHA1   SigAlgo = "RSASSA_PSS_SHA_1"
	SigAlgoRsaPssSHA224 SigAlgo = "RSASSA_PSS_SHA_224"
	SigAlgoRsaPssSHA256 SigAlgo = "RSASSA_PSS_SHA_256"
	SigAlgoRsaPssSHA384 SigAlgo = "RSASSA_PSS_SHA_384"
	SigAlgoRsaPssSHA512 SigAlgo = "RSASSA_PSS_SHA_512"
	// SigAlgoRsaPssSHA512_224  SigAlgo = "RSASSA_PSS_SHA_512_224"
	// SigAlgoRsaPssSHA512_256  SigAlgo = "RSASSA_PSS_SHA_512_256"
	SigAlgoRsaPkcsMD5    SigAlgo = "RSASSA_PKCS1_V1_5_MD5"
	SigAlgoRsaPkcsSHA1   SigAlgo = "RSASSA_PKCS1_V1_5_SHA_1"
	SigAlgoRsaPkcsSHA224 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_224"
	SigAlgoRsaPkcsSHA256 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_256"
	SigAlgoRsaPkcsSHA384 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_384"
	SigAlgoRsaPkcsSHA512 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_512"
	// SigAlgoRsaPkcsSHA512_224 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_512_224"
	// SigAlgoRsaPkcsSHA512_256 SigAlgo = "RSASSA_PKCS1_V1_5_SHA_512_256"

	// rfc3279 rfc5758

	SigAlgoEcdsaSHA1   SigAlgo = "ECDSA_SHA_1"
	SigAlgoEcdsaSHA224 SigAlgo = "ECDSA_SHA_224"
	SigAlgoEcdsaSHA256 SigAlgo = "ECDSA_SHA_256"
	SigAlgoEcdsaSHA384 SigAlgo = "ECDSA_SHA_384"
	SigAlgoEcdsaSHA512 SigAlgo = "ECDSA_SHA_512"
)

var (
	// ErrUnsupportedHash is returned by Signer.Sign() when the provided hash
	// algorithm isn't supported.
	ErrUnsupportedHash = fmt.Errorf("unsupported hash algorithm")
)

type SignerOpts struct {
	Algorithm SigAlgo
	Context   context.Context
}

func (o *SignerOpts) HashFunc() crypto.Hash {
	switch o.Algorithm {
	case SigAlgoRsaPkcsMD5:
		return crypto.MD5
	case SigAlgoRsaPssSHA1, SigAlgoRsaPkcsSHA1, SigAlgoEcdsaSHA1:
		return crypto.SHA1
	case SigAlgoRsaPssSHA224, SigAlgoRsaPkcsSHA224, SigAlgoEcdsaSHA224:
		return crypto.SHA224
	case SigAlgoRsaPssSHA256, SigAlgoRsaPkcsSHA256, SigAlgoEcdsaSHA256:
		return crypto.SHA256
	case SigAlgoRsaPssSHA384, SigAlgoRsaPkcsSHA384, SigAlgoEcdsaSHA384:
		return crypto.SHA384
	case SigAlgoRsaPssSHA512, SigAlgoRsaPkcsSHA512, SigAlgoEcdsaSHA512:
		return crypto.SHA512
		// case SigAlgoRsaPssSHA512_224, SigAlgoRsaPkcsSHA512_224:
		// 	return crypto.SHA512_224
		// case SigAlgoRsaPssSHA512_256, SigAlgoRsaPkcsSHA512_256:
		// 	return crypto.SHA512_256
	}
	return 0
}

type RemoteSigner interface {
	Sign(ctx context.Context, digest []byte, algo SigAlgo) ([]byte, error)
	Public() crypto.PublicKey
}

type inst struct {
	impl RemoteSigner
}

func New(remote RemoteSigner) crypto.Signer {
	return &inst{
		impl: remote,
	}
}

func (v *inst) Public() crypto.PublicKey {
	return v.impl.Public()
}

func (v *inst) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	hash := opts.HashFunc()
	if len(digest) != hash.Size() {
		return nil, fmt.Errorf("bad digest for hash")
	}

	var algo SigAlgo
	var ctx context.Context

	switch opt := opts.(type) {
	case *SignerOpts:
		algo = opt.Algorithm
		ctx = opt.Context
	case *rsa.PSSOptions:
		switch hash {
		case crypto.SHA1:
			algo = SigAlgoRsaPssSHA1
		case crypto.SHA224:
			algo = SigAlgoRsaPssSHA224
		case crypto.SHA256:
			algo = SigAlgoRsaPssSHA256
		case crypto.SHA384:
			algo = SigAlgoRsaPssSHA384
		case crypto.SHA512:
			algo = SigAlgoRsaPssSHA512
		// case crypto.SHA512_224:
		// 	algo = SigAlgoRsaPssSHA512_224
		// case crypto.SHA512_256:
		// 	algo = SigAlgoRsaPssSHA512_256
		default:
			return nil, ErrUnsupportedHash
		}

	default:
		switch hash {
		case crypto.MD5:
			algo = SigAlgoRsaPkcsMD5
		case crypto.SHA1:
			algo = SigAlgoRsaPkcsSHA1
		case crypto.SHA224:
			algo = SigAlgoRsaPkcsSHA224
		case crypto.SHA256:
			algo = SigAlgoRsaPkcsSHA256
		case crypto.SHA384:
			algo = SigAlgoRsaPkcsSHA384
		case crypto.SHA512:
			algo = SigAlgoRsaPkcsSHA512
		// case crypto.SHA512_224:
		// 	algo = SigAlgoRsaPkcsSHA512_224
		// case crypto.SHA512_256:
		// 	algo = SigAlgoRsaPkcsSHA512_256
		default:
			return nil, ErrUnsupportedHash
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	return v.impl.Sign(ctx, digest, algo)
}
