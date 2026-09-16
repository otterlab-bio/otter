rule seqkit:
  message:"seqkit ..."
  input:
    fastq_R1 = lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R1.fastq.gz"),
    fastq_R2 = lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R2.fastq.gz"),
    trim_R1 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_1.fq.gz"),
    trim_R2 = lambda wildcards: os.path.join(config["output"]["trim_dir"], f"{wildcards.sample}_val_2.fq.gz")
  output:
    os.path.join(config["directories"]["qc"]["main"], "{sample}_seqkit_stat.txt")
  params:
    stat = lambda wildcards:os.path.join(config["directories"]["qc"]["main"], f"{wildcards.sample}_seqkit_stat.txt")
  threads:6
  shell:
    """
    enva run seqkit -- seqkit stat  -a -j {threads} -T -b {input.fastq_R1} {input.fastq_R2} {input.trim_R1} {input.trim_R2} > {params.stat}
    """
