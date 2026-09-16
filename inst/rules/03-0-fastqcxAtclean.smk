rule fastqcxAtclean:
  message:"Running fastqcx after Trim ..."
  input:
    R1 = os.path.join(config["output"]["trim_dir"], "{sample}_val_1.fq.gz"),
    R2 = os.path.join(config["output"]["trim_dir"], "{sample}_val_2.fq.gz")
  output:
    R1_data = os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqcx", "fastqc_data.txt"),
    R2_data = os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqcx", "fastqc_data.txt")
  params:
    R1_dir = os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqcx"),
    R2_dir = os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqcx")
  threads: 2
  shell:
    """
    fastqcx -q {input.R1:q} -s {params.R1_dir:q} --no-html
    fastqcx -q {input.R2:q} -s {params.R2_dir:q} --no-html
    """
