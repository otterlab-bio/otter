rule rnaseq_step1_checker :
  message:"Checking step1 of RNAseq ..."
  input:
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqc.zip"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqc.zip"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqc.html"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqc.html"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_val_1.fq.gz"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_val_2.fq.gz"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R1.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R2.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqc.zip"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqc.zip"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqc.html"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqc.html"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["main"], "{sample}_seqkit_stat.txt"), sample=config["metadata"]["sample_ids"]) 
  output:
    os.path.join(config["directories"]["sid_log"], "step1_success.txt")
  params:
    log_marker = os.path.join(config["directories"]["sid_log"], "step1_success.txt"),
    jobid = config["workflow"]["jobid"],
    user_email = config["metadata"]["user_email"]
  threads:5
  shell:
    """
    enva run base Rscript R/beaver_mail.R --step 1 --jobid {params.jobid} --send_to {params.user_email} 
    touch {params.log_marker}
    """
