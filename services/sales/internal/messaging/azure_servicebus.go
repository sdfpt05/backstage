package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"example.com/backstage/services/sales/config"
	
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// ServiceBusClient is an interface for Azure Service Bus operations
type ServiceBusClient interface {
	SendMessage(ctx context.Context, body interface{}) error
	Close() error
}

// serviceBusClient implements the ServiceBusClient interface
type serviceBusClient struct {
	client     *azservicebus.Client
	sender     *azservicebus.Sender
	queueName  string
	clientType string
}

// NewServiceBusClient creates a new Azure Service Bus client
func NewServiceBusClient(cfg config.ServiceBusConfig, clientType string) (ServiceBusClient, error) {
	if cfg.ConnectionString == "" {
		return nil, fmt.Errorf("Azure Service Bus connection string is empty")
	}
	
	// Create the Service Bus client
	client, err := azservicebus.NewClientFromConnectionString(cfg.ConnectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Service Bus client: %w", err)
	}
	
	// Create a sender for the queue
	sender, err := client.NewSender(cfg.QueueName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Service Bus sender: %w", err)
	}
	
	return &serviceBusClient{
		client:     client,
		sender:     sender,
		queueName:  cfg.QueueName,
		clientType: clientType,
	}, nil
}

// SendMessage sends a message to the Service Bus queue
func (s *serviceBusClient) SendMessage(ctx context.Context, body interface{}) error {
	// Convert the body to JSON
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message body: %w", err)
	}
	
	// Create the message
	msg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"source": s.clientType,
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	}
	
	// Send the message
	return s.sender.SendMessage(ctx, msg, nil)
}

// Close closes the Service Bus client
func (s *serviceBusClient) Close() error {
	// Close the sender
	if s.sender != nil {
		if err := s.sender.Close(context.Background()); err != nil {
			return err
		}
	}
	
	// Close the client
	if s.client != nil {
		return s.client.Close(context.Background())
	}
	
	return nil
}
