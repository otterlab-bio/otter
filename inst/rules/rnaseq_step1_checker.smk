rule rnaseq_step1_checker :
  message:"Checking step1 of RNAseq ..."
  input:
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_val_1.fq.gz"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_val_2.fq.gz"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R1.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R2.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"])
  output:
    os.path.join(config["directories"]["sid_log"], "step1_success.txt")
  params:
    log_marker = os.path.join(config["directories"]["sid_log"], "step1_success.txt"),
    jobid = config["workflow"]["jobid"],
    user_email = config["metadata"]["user_email"]
  threads:5
  shell:
    """
    # Skip email notification, just create success marker
    touch {params.log_marker}
    """
