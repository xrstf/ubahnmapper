import board
import digitalio
import time

# This does NOT blink the LED on the blinka board,
# but just anything connected to G0.
# The LED on the blinka board cannot be toggled, it's
# hardwired into V_in.

led = digitalio.DigitalInOut(board.G0)
led.direction = digitalio.Direction.OUTPUT

while True:
   print("on...")
   led.value = True
   time.sleep(5)
   print("off...")
   led.value = False
   time.sleep(1)
