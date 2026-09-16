#!/bin/bash
#SBATCH -N 1
#SBATCH -n 1
#SBATCH -J beaverdown2_genome_prepare
#SBATCH -o beaverdown2_%j.out
#SBATCH -e beaverdown2_%j.err
#SBATCH --get-user-env
#SBATCH --cpus-per-task 20
#SBATCH --mem 200G

# 检测是否是Conda环境
if conda info --envs | grep -q "bismark"; then
    echo "Using Conda environment 'bismark' to run bismark_genome_preparation."
    # 如果是Conda环境，使用conda run调用
    conda run -n bismark bismark_genome_preparation --verbose inst/pdx/mouse
    conda run -n bismark bismark_genome_preparation --verbose inst/pdx/homo_sapiens
else
    echo "Using system-wide installation of bismark to run bismark_genome_preparation."
    # 如果不是Conda环境，直接调用系统中的bismark
    bismark_genome_preparation --verbose inst/pdx/mouse
    bismark_genome_preparation --verbose inst/pdx/homo_sapiens
fi

conda run -n star \
STAR --runThreadN 20 \
--runMode genomeGenerate \
--genomeDir inst/rnaseq/homo_sapiens \
--genomeFastaFiles inst/rnaseq/homo_sapiens/hg38.fa \
--sjdbGTFfile inst/rnaseq/homo_sapiens/hg38.ensGene.gtf \
--sjdbOverhang 100
