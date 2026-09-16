#!/bin/env python
"""
Date: 2022-04-08
Update：
2022-04-08  修复代码风格
"""

import sys
import argparse
import pyfastx


parser = argparse.ArgumentParser()
parser.add_argument('-I', "--fastqGZ", required=True,
					help="Input Fastq file.")
parser.add_argument('-O', "--outFile", required=True,
					help="Output file.")
parser.add_argument('-E', "--topReads", type=int, default=10000000, required=False,
					help="Number of reads used.")
parser.add_argument('-L', "--NumBase", type=int, default=3, required=False,
					help="Number of bases used.")
args = parser.parse_args()

if args.NumBase == 0:
	sys.exit("NumBase must be 1 or larger.\n")


Letter = ['A', 'C', 'G', 'T', 'N']
countMap = dict()
# initialize CGG TGG count
countMap['CGG'] = 0
countMap['TGG'] = 0

if args.topReads == 0:
	args.topReads = float("inf")
# count NT
counter = 0
for name, seq, qual in pyfastx.Fastq(args.fastqGZ, build_index=False):
	if len(seq) < args.NumBase:
		continue
	subSeq = seq[0:args.NumBase]
	if subSeq not in countMap:
		countMap[subSeq] = 1
	else:
		countMap[subSeq] += 1
	counter += 1
	if counter >= args.topReads:
		break

OUT = open(args.outFile, 'w')
CCGG_count = countMap['CGG'] + countMap['TGG']
CCGG_ratio = (countMap['CGG'] + countMap['TGG']) / counter
OUT.write("CGG_and_TGG" + "\t" + str(CCGG_count) + "\t" + str(CCGG_ratio) + '\n')
for subSeq in countMap:
	ratio = countMap[subSeq] / args.topReads
	OUT.write(subSeq + '\t' + str(countMap[subSeq]) + "\t" + str(ratio) + '\n')
OUT.close()
