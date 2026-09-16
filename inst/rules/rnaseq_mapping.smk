rule rnaseqmappingbowtie:
  message: "RNAseq mapping ..."
  input:
    trim_R1 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_1.fq.gz"),
    trim_R2 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_2.fq.gz")
  output:
    sorted_bam=os.path.join(config["directories"]["bsmap"]["main"], "{sample}_{species}" + ".bam"),
    star_log=os.path.join(config["directories"]["bsmap"]["main"], "{species}", "{sample}Log.final.out")
  params:
    bsmapDir = config["directories"]["bsmap"]["main"],
    tempdir = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],"tmp", f"{wildcards.sample}"),
    sam_aligned = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],f"{wildcards.species}", f"{wildcards.sample}_aligned.sam"),
    bam_aligned = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],f"{wildcards.species}", f"{wildcards.sample}Aligned.sortedByCoord.out.bam"),
    bam_aligned_prefix = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],f"{wildcards.species}", f"{wildcards.sample}"),
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+f"{wildcards.species}"+".bam"),
    rnaseq_ref = lambda wildcards:config["reference"]["rnaseq"]["ref"][config["workflow"]["species"]["name"].index(wildcards.species)]
  threads: 40
  shell:
    """
    enva run otter-core -- STAR --runThreadN {threads} \
    --readFilesCommand zcat \
    --quantMode GeneCounts \
    --genomeDir {params.rnaseq_ref} \
    --readFilesIn {input.trim_R1} {input.trim_R2} \
    --twopassMode Basic \
    --outSAMunmapped None \
    --outSAMtype BAM SortedByCoordinate \
     --outSAMattributes NH HI AS nM NM MD \
    --outFileNamePrefix  {params.bam_aligned_prefix} 
    
    mv {params.bam_aligned} {params.bam_sorted}
    
    enva run otter-core -- samtools index -@ 8 {params.bam_sorted}
    
    """
