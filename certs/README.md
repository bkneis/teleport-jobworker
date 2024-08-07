# TLS for job worker

Below are the commands used to generate the self signed mTLS certs. `github.com/cloudflare/cfssl` has been used to wrap the openssl commands to make it easier to generate the self signed certs. TODO in production these would need to be signed by a real root CA and loaded into memory securely.

```
go install github.com/cloudflare/cfssl/cmd/...@latest

cfssl selfsign -config cfssl.json --profile rootca "Dev Testing CA" csr.json | cfssljson -bare root

cfssl genkey csr.json | cfssljson -bare server
cfssl genkey csr.json | cfssljson -bare client
cfssl genkey csr2.json | cfssljson -bare client
cfssl sign -ca root.pem -ca-key root-key.pem -config cfssl.json -profile server server.csr | cfssljson -bare server
cfssl sign -ca root.pem -ca-key root-key.pem -config cfssl.json -profile client client.csr | cfssljson -bare client
cfssl sign -ca root.pem -ca-key root-key.pem -config cfssl.json -profile client client2.csr | cfssljson -bare client
```

The directory is also a package in `jobworker` and contains `data.go`, this provides a convenient function to get an absolute file path to the certificates, inspired by https://github.com/grpc/grpc-go/tree/master/examples/data
