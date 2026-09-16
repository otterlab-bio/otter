rule bismarkreport:
  message:"building bismark report ..."
  input:
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}_nsort.bismark.cov.gz"),sample = config["metadata"]["sample_ids"]),
    graft_align = expand(os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"],"{sample}_val_1_bismark_bt2_pe.bam"),sample = config["metadata"]["sample_ids"]),
  output:
    os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"], "{sample}.html")
  params:
    outDir = os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"]),
    alignment_log = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"], config["workflow"]["species"]["graft"],f"{wildcards.sample}_val_1_bismark_bt2_PE_report.txt"),
    split_log = lambda wildcards:os.path.join(config["directories"]["methylation_call"],f"{wildcards.sample}_nsort_splitting_report.txt"),
    mbias_log = lambda wildcards:os.path.join(config["directories"]["methylation_call"],f"{wildcards.sample}_nsort.M-bias.txt"),
    samplename = lambda wildcards:f"{wildcards.sample}.html"
    
  threads:1
  shell:
    """
    bismark2report --dir {params.outDir} \
    --output {params.samplename} \
    --alignment_report {params.alignment_log} \
    --splitting_report {params.split_log} \
    --mbias_report {params.mbias_log}
    
    """
