package main

import (
	acr122u "github.com/kurrik/acr122u"
	nats "github.com/nats-io/go-nats"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		panic(err)
	}

	ctx, err := acr122u.EstablishContext()
	if err != nil {
		panic(err)
	}

	ctx.ServeFunc(func(c acr122u.Card) {
		nc.Publish("acr122u", c.UID())
	})
}
