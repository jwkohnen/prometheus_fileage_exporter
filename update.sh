#!/bin/bash

DATE="date --rfc-3339=ns"

while (true); do
	$DATE > /var/tmp/start
	sleep $((RANDOM % 8))
	$DATE > /var/tmp/end
	sleep 15
done
