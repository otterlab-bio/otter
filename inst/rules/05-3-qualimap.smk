rule qualimap:
  message: "qualimap ..."
  input:
    sample_bam = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_{wildcards.species}.bam")
  output:
    html=os.path.join(config["directories"]["qualimap"],"{sample}_{species}", "qualimapReport.html"),
    pdf=os.path.join(config["directories"]["qualimap"],"{sample}_{species}", "report.pdf"),
    genome_results=os.path.join(config["directories"]["qualimap"],"{sample}_{species}", "genome_results.txt")
  params:
    outdir_qualimap = lambda wildcards:os.path.join(config["directories"]["qualimap"], f"{wildcards.sample}_{wildcards.species}"),
    java_mem = "40G"
  threads:2
  shell:
    """
    export JAVA_OPTS="-Djava.awt.headless=true"
    enva run otter-core -- qualimap bamqc -bam {input.sample_bam} -outdir {params.outdir_qualimap} -outformat PDF:HTML  --java-mem-size={params.java_mem}
    """
