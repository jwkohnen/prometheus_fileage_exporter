#!/bin/bash

DATE="date --rfc-3339=ns"

while (true); do
	$DATE > start
	sleep $((RANDOM % 5))
	$DATE > end
	sleep 5
done
