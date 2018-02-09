package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/atotto/clipboard"

	"strconv"

	cloudstore "cloud.google.com/go/storage"
	"github.com/emicklei/tre"
	"golang.org/x/net/context"
	"google.golang.org/api/cloudkms/v1"
)

var version = "v1.3.3"

func main() {
	flag.Parse()
	loadConfiguration()
	if *oVersion {
		fmt.Println("kiya version", version)
		os.Exit(0)
	}
	if len(flag.Args()) < 2 {
		fmt.Println("kiya [flags] [profile] [get|put|delete|list|template|copy|paste|move|generate] [|parent/key] [|value] [|template-filename] [|secret-length]")
		fmt.Println("    if value, template-filename or secret length is needed, but missing, it is read from stdin")
		flag.PrintDefaults()
		os.Exit(0)
	}
	// Create the KMS client.
	kmsService, err := cloudkms.New(newAuthenticatedClient())
	if err != nil {
		log.Fatal(err)
	}
	// Create the Bucket client
	storageService, err := cloudstore.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create client [%v]", err)
	}
	profileName := flag.Arg(0)
	target, ok := profiles[profileName]
	if !ok {
		log.Fatalf("no such profile [%s] please check your .kiya file", profileName)
	}
	// what command?
	switch flag.Arg(1) {

	case "put":
		key := flag.Arg(2)
		var value string
		if len(flag.Arg(3)) != 0 {
			value = flag.Arg(3)
		} else {
			value = readFromStdIn()
		}
		commandPutPasteGenerate(kmsService, storageService, target, "put", key, value)

	case "paste":
		key := flag.Arg(2)
		value, err := clipboard.ReadAll()

		if err != nil {
			log.Fatal(tre.New(err, "clipboard read failed", "key", key))
		}
		commandPutPasteGenerate(kmsService, storageService, target, "paste", key, value)

	case "generate":
		key := flag.Arg(2)
		var length string
		if len(flag.Arg(3)) != 0 {
			length = flag.Arg(3)
		} else {
			length = readFromStdIn()
		}

		secretLength, err := strconv.Atoi(length)
		if err != nil {
			log.Fatal(tre.New(err, "generate failed", "key", key, "err", err))
		}
		secret, err := GenerateSecret(secretLength, target.SecretRunes)
		if err != nil {
			log.Fatal(tre.New(err, "generate failed", "key", key, "err", err))
		}
		commandPutPasteGenerate(kmsService, storageService, target, "generate", key, secret)

	case "copy":
		key := flag.Arg(2)
		value, err := getValueByKey(kmsService, storageService, key, target)
		if err != nil {
			log.Fatal(tre.New(err, "get failed", "key", key, "err", err))
		}
		if err := clipboard.WriteAll(value); err != nil {
			log.Fatal(tre.New(err, "copy failed", "key", key, "err", err))
		}

	case "get":
		key := flag.Arg(2)
		value, err := getValueByKey(kmsService, storageService, key, target)
		if err != nil {
			log.Fatal(tre.New(err, "get failed", "key", key, "err", err))
		}
		if len(*oOutputFilename) > 0 {
			if err := ioutil.WriteFile(*oOutputFilename, []byte(value), os.ModePerm); err != nil {
				log.Fatal(tre.New(err, "get failed", "key", key, "err", err))
			}
			return
		}
		fmt.Println(value)

	case "delete":
		key := flag.Arg(2)
		commandDelete(kmsService, storageService, target, key)
	case "list":
		commandList(storageService, target)
	case "template":
		commandTemplate(kmsService, storageService, target)
	case "move":
		// kiya [source] move [source-key] [target] [|target-key]
		sourceProfile := profiles[flag.Arg(0)]
		sourceKey := flag.Arg(2)
		targetProfile := profiles[flag.Arg(3)]
		targetKey := sourceKey
		if len(flag.Args()) == 5 {
			targetKey = flag.Arg(4)
		}
		commandMove(kmsService, storageService, sourceProfile, sourceKey, targetProfile, targetKey)
	default:
		fmt.Println("unknown command", flag.Arg(1))
	}
}
