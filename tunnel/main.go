package main

import (
	"context"
	"log"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/songgao/water"
)

func makeTun(ifce_name string) *water.Interface {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = ifce_name

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
	}
	return ifce
}

const RAW_INSERT_QUERY = `
INSERT INTO raw_packet (payload, sender) VALUES ($1, $2)
RETURNING id
`

func configureInterface(ifce_name string) {
	ip_addr_cmd := exec.Command("ip", "addr", "add", os.Getenv("TUN_IP"), "dev", ifce_name)
	ip_addr_cmd.Run()
	ip_link_cmd := exec.Command("ip", "link", "set", "up", "dev", ifce_name)
	ip_link_cmd.Run()
}

func pollTunPackets(ifce *water.Interface, dbPool *pgxpool.Pool) {
	localIP := os.Getenv("TUN_IP")
	packet := make([]byte, 2000)
	for {
		n, err := ifce.Read(packet)
		if err != nil {
			log.Fatal(err)
		}
		if packet[0] != 0x45 {
			log.Printf("Skipping non-IP packet")
			continue
		}

		_, err = dbPool.Exec(context.Background(), RAW_INSERT_QUERY, packet[:n], localIP)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func postgresListen(listen_conn *pgx.Conn, dbPool *pgxpool.Pool, ifce *water.Interface) {
	var packet []byte
	peer := os.Getenv("TUN_PEER")
	listen_conn.Exec(context.Background(), "listen raw_packet_ready")
	for {
		// TODO: this should use a connection from the pool but the function doesn't
		// seem defined for those types of connections ¯\_(ツ)_/¯
		_, err := listen_conn.WaitForNotification(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		rows, _ := dbPool.Query(
			context.Background(),
			"SELECT id, payload, length(payload) FROM raw_packet WHERE host(sender) = $1 AND received = 0 LIMIT 50",
			peer,
		)
		defer rows.Close()

		counter := 0
		for rows.Next() {
			counter++

			var id int
			var length int
			err := rows.Scan(
				&id,
				&packet,
				&length,
			)
			if err != nil {
				log.Fatal(err)
			}
			_, err = ifce.Write(packet[:length])
			if err != nil {
				log.Fatal(err)
			}
			_, err = dbPool.Exec(context.Background(), "UPDATE raw_packet SET received = 1 WHERE id = $1", id)
			if err != nil {
				log.Fatal(err)
			}
		}
		if counter > 1 {
			log.Printf("%d\n", counter)
		}
	}

}

func main() {
	ifce := makeTun("postgres-vpn")
	log.Printf("Interface Name: %s\n", ifce.Name())
	configureInterface(ifce.Name())

	database_url := os.Getenv("DATABASE_URL")
	dbPool, err := pgxpool.New(context.Background(), database_url)
	if err != nil {
		log.Fatal(err)
	}
	defer dbPool.Close()

	listen_conn, err := pgx.Connect(context.Background(), database_url)
	if err != nil {
		log.Fatal(err)
	}
	defer listen_conn.Close(context.Background())

	go pollTunPackets(ifce, dbPool)
	postgresListen(listen_conn, dbPool, ifce)

}
