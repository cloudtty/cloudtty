package grpcsigner

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/tg123/remotesigner"
)

type wrapinst struct {
	grpc     SignerClient
	metadata string
}

var _ remotesigner.RemoteSigner = (*wrapinst)(nil)

func New(client SignerClient, metadata string) remotesigner.RemoteSigner {
	return &wrapinst{
		grpc:     client,
		metadata: metadata,
	}
}

func (g *wrapinst) Sign(ctx context.Context, digest []byte, algo remotesigner.SigAlgo) ([]byte, error) {
	reply, err := g.grpc.Sign(ctx, &SignRequest{
		Digest:    digest,
		Algorithm: string(algo),
		Metadata:  g.metadata,
	})

	if err != nil {
		return nil, err
	}

	return reply.Signature, nil
}

func (g *wrapinst) Public() crypto.PublicKey {
	reply, err := g.grpc.PublicKey(context.Background(), &PublicKeyRequest{
		Metadata: g.metadata,
	})

	if err != nil {
		return nil
	}

	switch reply.Type {
	case "PKIX":
		p, err := x509.ParsePKIXPublicKey(reply.Data)
		if err != nil {
			return nil
		}

		return p
	case "PKCS1":
		p, err := x509.ParsePKCS1PublicKey(reply.Data)
		if err != nil {
			return nil
		}
		return p
	}

	return nil
}

type SignerFactory func(metadata string) (crypto.Signer, error)

type server struct {
	UnimplementedSignerServer
	factory SignerFactory
}

func NewSignerServer(factory SignerFactory) (SignerServer, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory must not be nil")
	}

	return &server{
		factory: factory,
	}, nil
}

func (s *server) Sign(_ context.Context, req *SignRequest) (*SignReply, error) {
	signer, err := s.factory(req.Metadata)
	if err != nil {
		return nil, err
	}

	sig, err := signer.Sign(rand.Reader, req.Digest, &remotesigner.SignerOpts{
		Algorithm: remotesigner.SigAlgo(req.Algorithm),
	})

	if err != nil {
		return nil, err
	}

	return &SignReply{
		Signature: sig,
	}, nil
}

func (s *server) PublicKey(_ context.Context, req *PublicKeyRequest) (*PublicKeyReply, error) {
	signer, err := s.factory(req.Metadata)
	if err != nil {
		return nil, err
	}
	
	p := signer.Public()

	k, ok := p.(*rsa.PublicKey)

	if ok {
		return &PublicKeyReply{
			Data: x509.MarshalPKCS1PublicKey(k),
			Type: "PKCS1",
		}, nil
	}

	data, err := x509.MarshalPKIXPublicKey(p)

	if err != nil {
		return nil, err
	}

	return &PublicKeyReply{
		Data: data,
		Type: "PKIX",
	}, nil
}
