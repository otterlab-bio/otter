rule fastqcxAtfirst:
  message:"Running fastqcx At first Glance ..."
  input:
    R1 = lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R1.fastq.gz"),
    R2 = lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R2.fastq.gz")
  output:
    R1_data = os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqcx", "fastqc_data.txt"),
    R2_data = os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqcx", "fastqc_data.txt")
  params:
    R1_dir = os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqcx"),
    R2_dir = os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqcx")
  threads: 2
  shell:
    """
    fastqcx -q {input.R1:q} -s {params.R1_dir:q} --no-html
    fastqcx -q {input.R2:q} -s {params.R2_dir:q} --no-html
    """
