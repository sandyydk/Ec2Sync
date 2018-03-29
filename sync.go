package main

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	s3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	ssh "golang.org/x/crypto/ssh"
)

var (
	Server_SSH_KeyLocation = "/tmp/sshkey.pem" //Used to store pem file in local memory
	REGION                 = "us-east-1"
)

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, s3Event events.S3Event) {
	CCM_IP, ok := os.LookupEnv("CCM_IP")
	if !ok {
		log.Println("CCM_IP not found")
		return
	}

	pemfile, ok := os.LookupEnv("PEM_FILE")
	if !ok {
		log.Println("PEM file name not defined in env")
		return
	}

	REGION, ok = os.LookupEnv("REGION")
	if !ok {
		log.Println("REGION not set in env. Setting it to us-east-1")
		REGION = "us-east-1"
	}

	username, ok := os.LookupEnv("USERNAME")
	if !ok {
		log.Println("USERNAME not defined in env")
		return
	}
	for _, record := range s3Event.Records {
		s3Handle := record.S3
		bucket := s3Handle.Bucket.Name
		objKey := s3Handle.Object.Key

		r := strings.HasSuffix(objKey, ".pem")
		if r {
			log.Println("Skipping as it's a pem file: %v", objKey)
			continue
		}
		log.Printf("[%s - %s] Bucket = %s, Key = %s \n", record.EventSource, record.EventTime, bucket, objKey)

		sess := session.Must(
			session.NewSession(
				&aws.Config{
					// Set supported region
					Region: aws.String(REGION),
				},
			),
		)

		downloader := s3manager.NewDownloader(sess)

		pem := &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &pemfile,
		}

		outfile, err := os.Create(Server_SSH_KeyLocation)
		if err != nil {
			log.Println("Error creating file")
			log.Println(err)
		}
		_, err = downloader.Download(outfile, pem)
		if err != nil {
			log.Println("Error downloading pem file")
			log.Println(err)
		}

		svc := s3.New(sess)

		input := &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &objKey,
		}

		_, err = svc.GetObject(input)
		if err != nil {
			log.Printf("Failed: Create Job, %v\n", err)
			return
		}

		pemBytes, err := ioutil.ReadFile(Server_SSH_KeyLocation)
		if err != nil {
			log.Println("Error reading pem file")
			log.Fatal(err)
		}

		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			log.Fatalf("Parse key failed:%v", err)
		}

		config := &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Timeout: time.Second * 15,
		}

		conn, err := ssh.Dial("tcp", CCM_IP+":22", config)
		if err != nil {
			log.Fatalf("Dial failed:%v", err)
		}

		defer conn.Close()

		session, err := conn.NewSession()
		if err != nil {
			log.Fatalf("Session failed:%v", err)
		}

		defer session.Close()

		stdoutPipe, err := session.StdoutPipe()

		outputReader := io.Reader(stdoutPipe)
		outputScanner := bufio.NewScanner(outputReader)

		outputLine := make(chan string)
		Done := make(chan bool)
		// Array for future addition if required
		commands := []string{"sudo aws s3 sync s3://" + bucket + " /tmp --exclude=*.pem --exact-timestamps", "sleep 5"}
		log.Println(commands)
		command := strings.Join(commands, "; ")
		err = session.Run(command)
		if err != nil {
			if eerr, ok := err.(*ssh.ExitError); !ok {
				log.Println("Exit error: %v", eerr.Error())
				return
			}
			log.Println("Run generated error:%v", err)
		}

		go func(scan *bufio.Scanner, line chan string, done chan bool) {
			defer close(line)
			defer close(done)
			for scan.Scan() {
				line <- scan.Text()
			}
			done <- true
		}(outputScanner, outputLine, Done)

		running := true
		outputBuf := ""
		for running {
			select {
			case <-Done:
				running = false
			case line := <-outputLine:
				outputBuf += line + "\n"
			}
		}
		session.Close()

		// Success if execution reaches here
		log.Println(outputBuf)

		// TODO :- Add support for email to be sent

	}

}
