# RSVP 

RSVP is an events invitation platform that relies on physical QR Codes and allows printing, sending and tracking invitations to events.

## SSL Certificate Setup
This app supports HTTPS (TLS) with certificates for both local development and production.

### Local Development (localhost)
For local testing with trusted certificates, use mkcert.

Install mkcert:
```shell
brew install mkcert
mkcert -install
```

Generate certificates:

```shell
mkcert localhost 127.0.0.1 ::1
```

```shell
certs/localhost.pem
certs/localhost-key.pem
```

Set environment variables:

```shell
export TLS_CERT_PATH=certs/localhost.pem
export TLS_KEY_PATH=certs/localhost-key.pem
```

Production (public domain)
For production deployments using a real domain (rsvp.mprlab.com), use Let's Encrypt.

Steps:
On your Mac, install Certbot:

```shell
brew install certbot
```

Obtain a certificate via DNS challenge:

```shell
sudo certbot certonly --manual --preferred-challenges dns -d rsvp.mprlab.com
```

After success, certificates are stored in:

```shell
/etc/letsencrypt/live/mywebsite.com/fullchain.pem
/etc/letsencrypt/live/mywebsite.com/privkey.pem
```

Copy the certificates to the production server:

```shell
scp /etc/letsencrypt/live/mywebsite.com/fullchain.pem user@server:/opt/myapp/certs/fullchain.pem
scp /etc/letsencrypt/live/mywebsite.com/privkey.pem user@server:/opt/myapp/certs/privkey.pem
```

On the production server, set environment variables:

```shell
export TLS_CERT_PATH=/opt/myapp/certs/fullchain.pem
export TLS_KEY_PATH=/opt/myapp/certs/privkey.pem
```
