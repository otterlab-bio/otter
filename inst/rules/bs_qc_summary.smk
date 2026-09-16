rule qcsummary:
  message:"summary QC in the pipeline ..."
  input:
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R1.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["output"]["trim_dir"], "{sample}_R2.fastq.gz_trimming_report.txt"),sample = config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R1_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["before"], "{sample}_R2_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_1_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qc"]["after"], "{sample}_val_2_fastqcx", "fastqc_data.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["bsmap"]["main"], config["workflow"]["species"]["graft"], "{sample}_val_1_bismark_bt2_PE_report.txt"), sample=config["metadata"]["sample_ids"]),
    expand(os.path.join(config["directories"]["qualimap"], "{sample}_" + config["workflow"]["species"]["graft"], "genome_results.txt"), sample=config["metadata"]["sample_ids"]),
    os.path.join(config["directories"]["methylation_call"], "methrixh5", "CpG_coverage.xlsx"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5", "CpG_annotation_report.xlsx"),
    config_file=config["directories"]["qctb_config"]
  output:
    summary=os.path.join(config["directories"]["qc_summary"],"qc_summary.xlsx")
  threads:5
  shell:
    """
    qctb --config {input.config_file:q} --output {output.summary:q}
    """
    
