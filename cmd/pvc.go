/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
)

var (
	etcdCA   string
	etcdCert string
	etcdKey  string
	etcdHost string
	etcdPort string
)

func etcdClient() (*clientv3.Client, error) {
	ca, err := os.ReadFile(etcdCA)
	if err != nil {
		return nil, fmt.Errorf("read etcd CA: %w", err)
	}

	keyPair, err := tls.LoadX509KeyPair(etcdCert, etcdKey)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)

	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{fmt.Sprintf("%s:%s", etcdHost, etcdPort)},
		TLS: &tls.Config{
			RootCAs: certPool,
			Certificates: []tls.Certificate{
				keyPair,
			},
			InsecureSkipVerify: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new clien: %w", err)
	}
	return client, nil
}

// pvcCmd represents the pvc command
var pvcCmd = &cobra.Command{
	Use:   "pvc",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("etcdca: %s etcdkey: %s etcdcert: %s\n", etcdCA, etcdCert, etcdKey)

		client, err := etcdClient()
		if err != nil {
			log.Printf("Couldn't get client: %v", err)
			return
		}

		res, err := client.Get(context.Background(), "/registry/persistentvolumeclaims", clientv3.WithPrefix())
		if err != nil {
			log.Printf("Client get: %v", err)
			return
		}

		gvk := schema.GroupVersionKind{
			Group:   v1.GroupName,
			Version: "v1",
			Kind:    "PersistentVolumeClaim",
		}

		runtimeSchema := runtime.NewScheme()
		runtimeSchema.AddKnownTypeWithName(gvk, &v1.PersistentVolumeClaim{})
		protoSerializer := protobuf.NewSerializer(runtimeSchema, runtimeSchema)

		for _, kv := range res.Kvs {
			pvc := &v1.PersistentVolumeClaim{}

			_, _, err := protoSerializer.Decode(kv.Value, &gvk, pvc)
			if err != nil {
				log.Printf("Decode protobuf: %v", err)
				return
			}

			(*pvc).ObjectMeta.DeletionTimestamp = nil
			(*pvc).ObjectMeta.DeletionGracePeriodSeconds = nil

			var fixedPVC bytes.Buffer
			err = protoSerializer.Encode(pvc, &fixedPVC)
			if err != nil {
				log.Printf("Encode protobuf: %v", err)
				return
			}

			_, err = client.Put(context.Background(), fmt.Sprintf("/registry/persistentvolumeclaims/%s/%s", pvc.Namespace, pvc.Name),
				fixedPVC.String())
			if err != nil {
				log.Printf("Client put: %v", err)
				return
			}
		}
	},
}

func init() {

	pvcCmd.Flags().StringVarP(&etcdCA, "etcdca", "c", "", "etcdca specifies the CA files.")
	pvcCmd.Flags().StringVarP(&etcdCert, "etcdcert", "a", "", "etcdcert specifies the cert files.")
	pvcCmd.Flags().StringVarP(&etcdKey, "etcdkey", "k", "", "etcdkey specifies the key files.")
	pvcCmd.Flags().StringVarP(&etcdHost, "etcdhost", "o", "localhost", "etcdhost specifies the host.")
	pvcCmd.Flags().StringVarP(&etcdPort, "etcdport", "p", "2379", "etcdport specifies the port.")

	rootCmd.AddCommand(pvcCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pvcCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pvcCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
