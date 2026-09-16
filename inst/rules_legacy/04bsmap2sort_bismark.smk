rule bsmap2sort4homo:
  message: "bsmap ..."
  input:
    trim_R1 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_1.fq.gz"),
    trim_R2 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_2.fq.gz")
  output:
    os.path.join(config["directories"]["bsmap"]["main"], "{sample}_{species}" + ".bam")
  params:
    bsmapDir = config["directories"]["bsmap"]["main"],
    bsmapDir_species = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],f"{wildcards.species}"),
    bam_aligned = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"],f"{wildcards.species}", f"{wildcards.sample}_val_1_bismark_bt2_pe.bam"),
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+f"{wildcards.species}"+".bam"),
    bamTmp = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"],"tmp",f"{wildcards.species}"),
    genomeFile = lambda wildcards:config["reference"]["indices"]["genome"][config["workflow"]["species"]["name"].index(wildcards.species)]
  threads: 40
  shell:
    """
    bismark --genome {params.genomeFile} --nucleotide_coverage --parallel 8 -1 {input.trim_R1} -2 {input.trim_R2} -o {params.bsmapDir_species}  --temp_dir {params.bamTmp}
    
    enva run otter-core -- samtools sort -@ {threads} -o {params.bam_sorted} {params.bam_aligned}
    
    enva run otter-core -- samtools index -@ {threads} -b {params.bam_sorted}
    
    #rm -f {params.bam_aligned}
    
    """
