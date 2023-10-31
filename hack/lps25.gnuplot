set datafile separator ';'
set timefmt '%Y-%m-%dT%H:%M:%S'
set format x "%H:%M:%S"
set key autotitle columnhead
set xdata time
plot "< cat -" using 1:2 with lines
