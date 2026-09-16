rule rnaseq_step3_checker :
  message:"Checking step1 of RNAseq ..."
  input:
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}_nsort.bismark.cov.gz"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["bsmap"]["main"], "{sample}_nsort.bam"),sample =config["metadata"]["sample_ids"] ),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","assays.h5"),
    os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"], "bismark_summary_report.html"),
    os.path.join(config["directories"]["qc_summary"],"qc_summary.xlsx") 
  output:
    os.path.join(config["directories"]["sid_log"], "step3_success.txt")
  params:
    log_marker = os.path.join(config["directories"]["sid_log"], "step3_success.txt"),
    jobid = config["workflow"]["jobid"],
    user_email = config["metadata"]["user_email"]
  threads:5
  shell:
    """
    # Skip email notification, just create success marker
    touch {params.log_marker}
    """
