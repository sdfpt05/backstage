package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// FirmwareConfig holds configuration for firmware management
type FirmwareConfig struct {
	StoragePath       string  // Path to store firmware binaries
	KeysPath          string  // Path to crypto keys for signing
	SigningAlgorithm  string  // Signing algorithm (e.g., "secp256r1")
	PublicKeyFile     string  // Public key filename
	PrivateKeyFile    string  // Private key filename
	VerifySignatures  bool    // Whether to verify signatures
	RequireSignatures bool    // Whether signatures are required
}

// GetAbsoluteStoragePath returns the absolute path to firmware storage
func (c *FirmwareConfig) GetAbsoluteStoragePath() (string, error) {
	if filepath.IsAbs(c.StoragePath) {
		return c.StoragePath, nil
	}
	
	// Convert relative path to absolute
	absPath, err := filepath.Abs(c.StoragePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	return absPath, nil
}

// GetPublicKeyPath returns the absolute path to the public key file
func (c *FirmwareConfig) GetPublicKeyPath() (string, error) {
	path := filepath.Join(c.KeysPath, c.PublicKeyFile)
	
	if filepath.IsAbs(path) {
		return path, nil
	}
	
	// Convert relative path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	return absPath, nil
}

// GetPrivateKeyPath returns the absolute path to the private key file
func (c *FirmwareConfig) GetPrivateKeyPath() (string, error) {
	path := filepath.Join(c.KeysPath, c.PrivateKeyFile)
	
	if filepath.IsAbs(path) {
		return path, nil
	}
	
	// Convert relative path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	return absPath, nil
}

// LoadPublicKey loads the ECDSA public key from file
func (c *FirmwareConfig) LoadPublicKey() (*ecdsa.PublicKey, error) {
	path, err := c.GetPublicKeyPath()
	if err != nil {
		return nil, err
	}
	
	// Read the public key file
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}
	
	// Decode PEM block
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	
	// Parse public key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	
	// Check if it's an ECDSA public key
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not an ECDSA key")
	}
	
	return ecdsaPub, nil
}

// LoadPrivateKey loads the ECDSA private key from file
func (c *FirmwareConfig) LoadPrivateKey() (*ecdsa.PrivateKey, error) {
	path, err := c.GetPrivateKeyPath()
	if err != nil {
		return nil, err
	}
	
	// Read the private key file
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	
	// Decode PEM block
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	
	// Parse private key
	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	
	return priv, nil
}

// GetEllipticCurve returns the elliptic curve based on the configured algorithm
func (c *FirmwareConfig) GetEllipticCurve() elliptic.Curve {
	switch c.SigningAlgorithm {
	case "secp256r1", "P-256":
		return elliptic.P256()
	case "secp384r1", "P-384":
		return elliptic.P384()
	case "secp521r1", "P-521":
		return elliptic.P521()
	default:
		// Default to P-256 (secp256r1)
		return elliptic.P256()
	}
}

// CreateStorageDirIfNotExists creates the firmware storage directory if it doesn't exist
func (c *FirmwareConfig) CreateStorageDirIfNotExists() error {
	path, err := c.GetAbsoluteStoragePath()
	if err != nil {
		return err
	}
	
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory with permissions
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create firmware storage directory: %w", err)
		}
	}
	
	return nil
}

// CreateKeysDirIfNotExists creates the keys directory if it doesn't exist
func (c *FirmwareConfig) CreateKeysDirIfNotExists() error {
	// Check if directory exists
	if _, err := os.Stat(c.KeysPath); os.IsNotExist(err) {
		// Create directory with permissions
		if err := os.MkdirAll(c.KeysPath, 0700); err != nil {
			return fmt.Errorf("failed to create keys directory: %w", err)
		}
	}
	
	return nil
}