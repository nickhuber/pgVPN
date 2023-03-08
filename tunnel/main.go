package main

import (
	"context"
	"fmt"
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
const RAW_NOTIFY_QUERY = `
SELECT pg_notify('raw_packet_ready:%s', '%d')
`

func configureInterface(ifce_name string) {
	ip_addr_cmd := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/30", os.Getenv("TUN_IP")), "dev", ifce_name)
	ip_addr_cmd.Run()
	ip_link_cmd := exec.Command("ip", "link", "set", "up", "dev", ifce_name)
	ip_link_cmd.Run()
}

func pollTunPackets(ctx context.Context, ifce *water.Interface, dbPool *pgxpool.Pool) {
	localIP := os.Getenv("TUN_IP")
	for {
		packet := make([]byte, 2000)
		n, err := ifce.Read(packet)
		if err != nil {
			log.Fatal(err)
		}
		go handlePacketRead(ctx, packet[:n], localIP, dbPool)
	}
}

func handlePacketRead(ctx context.Context, packet []byte, localIP string, dbPool *pgxpool.Pool) {
	if packet[0] != 0x45 {
		log.Printf("Skipping non-IPv4 packet")
		return
	}

	var id int
	err := dbPool.QueryRow(ctx, RAW_INSERT_QUERY, packet, os.Getenv("TUN_IP")).Scan(&id)
	if err != nil {
		log.Fatal(err)
	}

	_, err = dbPool.Exec(ctx, fmt.Sprintf(RAW_NOTIFY_QUERY, os.Getenv("TUN_IP"), id))
	if err != nil {
		log.Fatal(err)
	}
}

func handlePostgresListen(row_id string, ctx context.Context, dbPool *pgxpool.Pool, ifce *water.Interface) {
	var payload []byte
	err := dbPool.QueryRow(ctx, "SELECT payload FROM raw_packet WHERE id = $1", row_id).Scan(&payload)
	if err != nil {
		log.Fatal(err)
	}

	_, err = ifce.Write(payload)
	if err != nil {
		log.Fatal(err)
	}
}

func postgresListen(ctx context.Context, dbPool *pgxpool.Pool, ifce *water.Interface) {
	peer := os.Getenv("TUN_PEER")
	listen_conn, err := dbPool.Acquire(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer listen_conn.Release()
	listen_cmd := fmt.Sprintf("listen \"raw_packet_ready:%s\"", peer)
	listen_conn.Exec(ctx, listen_cmd)
	for {
		notification, err := listen_conn.Conn().WaitForNotification(ctx)
		if err != nil {
			log.Fatal(err)
		}
		handlePostgresListen(notification.Payload, ctx, dbPool, ifce)
	}

}

func main() {
	ifce := makeTun("postgres-vpn")
	log.Printf("Interface Name: %s\n", ifce.Name())
	log.Printf("Interface IP: %s\n", os.Getenv("TUN_IP"))
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

	ctx := context.Background()
	go pollTunPackets(ctx, ifce, dbPool)
	postgresListen(ctx, dbPool, ifce)

}
