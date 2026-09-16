rule collectGCbias:
  message: "collectGCbias ..."
  input:
    sample_bam = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_{wildcards.species}.bam")
  output:
    os.path.join(config["directories"]["qc"]["main"],"GCbias","{sample}_{species}", "gc_bias_metrics.txt"),
    os.path.join(config["directories"]["qc"]["main"],"GCbias","{sample}_{species}", "gc_bias_metrics.pdf"),
    os.path.join(config["directories"]["qc"]["main"],"GCbias","{sample}_{species}", "summary_metrics.txt")
  params:
    outdir_gcbias = lambda wildcards:os.path.join(config["directories"]["qc"]["main"],"GCbias",f"{wildcards.sample}_{wildcards.species}"),
    fasta = lambda wildcards:config["reference"]["files"]["fasta"][config["workflow"]["species"]["name"].index(wildcards.species)],
    gc_txt = lambda wildcards:os.path.join(config["directories"]["qc"]["main"],"GCbias",f"{wildcards.sample}_{wildcards.species}","gc_bias_metrics.txt"),
    gc_pdf = lambda wildcards:os.path.join(config["directories"]["qc"]["main"],"GCbias",f"{wildcards.sample}_{wildcards.species}","gc_bias_metrics.pdf"),
    gc_sum = lambda wildcards:os.path.join(config["directories"]["qc"]["main"],"GCbias",f"{wildcards.sample}_{wildcards.species}","summary_metrics.txt")
  threads:4
  shell:
    """
    enva run picard -- picard CollectGcBiasMetrics \
      I={input.sample_bam} \
      O={params.gc_txt}  \
      CHART={params.gc_pdf}  \
      S={params.gc_sum}  \
      R={params.fasta} 
    """
