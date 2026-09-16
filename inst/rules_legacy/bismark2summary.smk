rule bismarksummary:
  message:"building bismark summary..."
  input:
    graft_align = expand(os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"],"{sample}_val_1_bismark_bt2_pe.bam"),sample = config["metadata"]["sample_ids"]),
    bismark_report = expand(os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"],"{sample}.html"),sample = config["metadata"]["sample_ids"])
  output:
    os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"], "bismark_summary_report.html")
  params:
    rundir = os.path.join(config["directories"]["bsmap"]["main"],config["workflow"]["species"]["graft"]),
    graft_align = expand("{sample}_val_1_bismark_bt2_pe.bam",sample = config["metadata"]["sample_ids"])
  threads:1
  shell:
    """
    cd {params.rundir} && enva run otter-core -- bismark2summary {params.graft_align}
    
    """
