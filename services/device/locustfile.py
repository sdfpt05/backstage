"""
Locust file for load testing device message endpoint.
Simulates 1000 devices sending messages every 10 seconds.
"""
import json
import random
import time
from locust import HttpUser, task, between
from datetime import datetime

class DeviceUser(HttpUser):
    # Wait between 8-12 seconds between tasks
    # This creates an average of 10 seconds with some natural variation
    wait_time = between(1, 2)
    
    def on_start(self):
        """Load the devices file and pick a random device to simulate"""
        try:
            with open("devices.json", "r") as f:
                self.devices = json.load(f)
            
            if not self.devices:
                print("No devices found in devices.json. Run create_devices.py first.")
                self.environment.runner.quit()
                return
                
            # Pick a random device to simulate from the list
            self.device = random.choice(self.devices)
            self.device_uid = self.device["uid"]
            print(f"Simulating device: {self.device['serial']} (UID: {self.device_uid})")
        except Exception as e:
            print(f"Error loading devices.json: {e}")
            print("Make sure to run create_devices.py first to generate the devices file.")
            self.environment.runner.quit()
    
    def generate_message(self):
        """Generate a realistic device message"""
        # Generate random but realistic values
        ram_usage = random.randint(20, 95)  # RAM usage in MB
        milliseconds = random.randint(1000000, 9999999)  # Processing time
        seconds_up = random.randint(1, 86400)  # Seconds up (up to 24 hours)
        memory_map = random.randint(10000, 99999)  # Memory map address
        
        # Create a message similar to the sample provided
        message = {
            "ev": "check",
            "rss": ram_usage,
            "ms": milliseconds,
            "s": seconds_up,
            "mm": memory_map
        }
        
        # Add timestamp and other fields occasionally
        if random.random() < 0.3:
            message["ts"] = int(time.time())
        
        if random.random() < 0.2:
            message["temp"] = round(random.uniform(35.5, 85.0), 2)
        
        return message
    
    @task
    def send_device_message(self):
        """Send a device message to the API"""
        message = self.generate_message()
        
        # Prepare the payload
        payload = {
            "device_uid": self.device_uid,
            "message": json.dumps(message),
            "sent_via": "locust-test"
        }
        
        # Record current time (for logging purposes)
        current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        
        # Send the message to the API
        with self.client.post(
            "/api/v1/devices/messages",
            json=payload,
            catch_response=True
        ) as response:
            # Validate the response
            if response.status_code == 200:
                response.success()
            else:
                error_msg = f"Failed to send message: {response.status_code} - {response.text}"
                response.failure(error_msg)
                print(f"[{current_time}] Device {self.device['serial']}: {error_msg}")