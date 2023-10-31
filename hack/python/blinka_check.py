import hid
import sys

print("Probing for Adafruit USB devices…")

ADAFRUIT_VENDOR_ID = 0x04D8

foundDevice = False
for device in hid.enumerate():
   if device["vendor_id"] == ADAFRUIT_VENDOR_ID:
      print(f"OK: Found Adafruit device with product ID {device['product_id']}!")
      foundDevice = True

if not foundDevice:
   print("Error: could not find Adafruit device(s).")
   sys.exit(1)

device = hid.device()
try:
   device.open(ADAFRUIT_VENDOR_ID, 0x00DD)
   print("OK: Successfully opened device.")
except OSError as e:
   print(f"Failed to open device: {e}")
   sys.exit(1)

print("Done.")
