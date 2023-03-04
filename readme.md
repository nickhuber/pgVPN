This is a simple tunnel that uses a Postgres database to transport packets from one peer to another. It works but is barely tested and isn't recommended for use.


To run simply run the application with the following environment variables set

`DATABASE_URL`, something like `postgresql://vpn:vpn@mydbhost:5432"`
`TUN_PEER`, an IP that is the peer IP of the tunnel
`TUN_IP`, the local IP of the tunnel

The performance sucks, and it isn't secure.

I also don't know Go very well, so the code probably sucks too.
