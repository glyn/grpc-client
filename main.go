package main

import (
	"google.golang.org/grpc"
	"fmt"
	"time"
	"golang.org/x/net/context"
	"github.com/glyn/grpc-client/pkg/function"
	"strconv"
	"io"
	"reflect"
	"os"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
)

func main() {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%v", 10382), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		panic(err)
	}

	client := function.NewMessageFunctionClient(conn)

	// Encode tests:
	//passed := testStream(client, []int{}, []int{})
	//passed = passed && testStream(client, []int{0}, []int{1, 0})
	//passed = passed && testStream(client, []int{0, 0, 0, 1, 2, 2}, []int{3, 0, 1, 1, 2, 2})
	//passed = passed && testStream(client, []int{0}, []int{1, 0})
	// Note: there is no way to send an error to the server via the input stream.

	// Decode tests:
	passed := testStream(client, []int{}, []int{})
	passed = passed && testStream(client, []int{1, 0}, []int{0})
	passed = passed && testStream(client, []int{3, 0, 1, 1, 2, 2}, []int{0, 0, 0, 1, 2, 2})
	passed = passed && testStream(client, []int{1, 0, 0, 1, 1, 2}, []int{0, 2})
	passed = passed && testStream(client, []int{1, 0, 1, 0}, []int{0, 0})
	passed = passed && testStream(client, []int{2, 1, 0}, []int{1, 1}, codes.InvalidArgument)
	passed = passed && testStream(client, []int{2, 1, -1, 0}, []int{1, 1}, codes.InvalidArgument)

	if !passed {
		os.Exit(1)
	}
}

func testStream(client function.MessageFunctionClient, input []int, expectedOutput []int, expectedErrorCodes ...codes.Code) bool {
	stream, err := client.Call(context.Background())
	if err != nil {
		panic(err)
	}
	sendBoundedIntStream(stream, input...)
	output, code := receiveBoundedIntStream(stream)
	if !reflect.DeepEqual(output, expectedOutput) {
		fmt.Printf("Failed. Received %v but expected %v\n", output, expectedOutput)
		return false
	}
	if len(expectedErrorCodes) == 0 {
		if code != codes.OK {
			fmt.Printf("Failed. Unexpected error %v\n", code)
			return false
		}
	} else {
		expected := false
		for _, e := range expectedErrorCodes {
			if code == e {
				expected = true
				break
			}
		}
		if !expected {
			fmt.Printf("Failed. Received error %v but expected one of %v\n", code, expectedErrorCodes)
			return false
		}
	}
	fmt.Println("Passed")
	return true
}

func sendBoundedIntStream(stream function.MessageFunction_CallClient, values ...int) {
	sendInt(stream, values...)
	err := stream.CloseSend()
	if err != nil {
		panic(err)
	}

}

func sendInt(stream function.MessageFunction_CallClient, values ...int) {
	for _, value := range values {
		err := stream.Send(&function.Message{
			Payload: []byte(fmt.Sprintf("%d", value)),
		})
		if err != nil {
			panic(err)
		}
	}
}

func receiveBoundedIntStream(stream function.MessageFunction_CallClient) ([]int, codes.Code) {
	ints := make([]int, 0, 10)
	for {
		i, err := receiveInt(stream)
		if err == io.EOF {
			return ints, codes.OK
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				return ints, status.Code()
			}
			panic(err)
		}
		ints = append(ints, i)
	}
}

func receiveInt(stream function.MessageFunction_CallClient) (int, error) {
	reply, err := stream.Recv()
	if err != nil {
		return 0, err
	}

	i, err := strconv.Atoi(string(reply.Payload))
	if err != nil {
		return 0, err
	}

	return i, nil
}
