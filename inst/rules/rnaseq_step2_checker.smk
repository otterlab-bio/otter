rule rnaseq_step2_checker :
  message:"Checking step1 of RNAseq ..."
  input:
    expand(os.path.join(config["directories"]["bsmap"]["main"], "{sample}_{species}.bam"), sample=config["metadata"]["sample_ids"],species = config["workflow"]["species"]["name"]) ,
    expand(os.path.join(config["directories"]["qualimap"],"{sample}_{species}","qualimapReport.html") , sample=config["metadata"]["sample_ids"],species = config["workflow"]["species"]["name"]) ,
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}_{species}"+".txt"), sample=config["metadata"]["sample_ids"],species = config["workflow"]["species"]["name"]),
    os.path.join(config["directories"]["beta_matrix"], "matrix_count.txt"),
    os.path.join(config["directories"]["beta_matrix"], "matrix_norm.txt") 
  output:
    os.path.join(config["directories"]["sid_log"], "step2_success.txt")
  params:
    log_marker = os.path.join(config["directories"]["sid_log"], "step2_success.txt"),
    jobid = config["workflow"]["jobid"],
    user_email = config["metadata"]["user_email"]
  threads:5
  shell:
    """
    # Skip email notification, just create success marker
    touch {params.log_marker}
    """
