#!python3 -u

import datetime
import board
import busio
import adafruit_lps2x
import time

i2c = busio.I2C(board.SCL, board.SDA)
lps = adafruit_lps2x.LPS25(i2c)

print("time;pressure")

while True:
   now = datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%S.%f")
   print(f"{now};{lps.pressure}")
   time.sleep(0.2)
