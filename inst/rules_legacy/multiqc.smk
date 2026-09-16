rule multiqc2summary:
  message: "multiqc ..."
  input:
    expand(os.path.join(config["directories"]["qualimap"],"{sample}_{species}","qualimapReport.html") , sample=SIDs,species = species),
    expand(os.path.join(config["directories"]["qualimap"],"{sample}_{species}","report.pdf") , sample=SIDs,species = species)
    
  output:
    os.path.join(config["directories"]["qc_summary"], "multiqc_report.html")
  params:
    outdir = config["directories"]["qc_summary"],
    readir = config["directories"]["work"]
  threads:4
  shell:
    """
    enva run multiqc -- multiqc {params.readir} -o {params.outdir} -f
    
    """
