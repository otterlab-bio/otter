#!/bin/bash

# 接受传入的参数
SAMPLE_BAM=$1
OUTPUT_FILE=$2

# 检查是否传入了足够的参数
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <path_to_bam_file> <output_file_path>"
    exit 1
fi

# 使用samtools和awk处理BAM文件，计算insert长度，并输出到文件
samtools view "$SAMPLE_BAM" | awk -F'\t' 'function abs(x){return ((x < 0.0) ? -x : x)} {print $1"\t"abs($9)}' | sort | uniq | cut -f2 > "$OUTPUT_FILE"
# 检查命令是否执行成功
if [ $? -eq 0 ]; then
    echo "Insert length calculation completed successfully."
else
    echo "Error occurred during insert length calculation."
    exit 1
fi