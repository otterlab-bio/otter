rule build_expression_matrix :
  message:"Build expression matrix ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+f"{wildcards.species}"+".bam")
  output:
    os.path.join(config["directories"]["methylation_call"], "{sample}_{species}.txt")
  params:
    rnaseq_gtf = lambda wildcards:config["reference"]["rnaseq"]["gtf"][config["workflow"]["species"]["name"].index(wildcards.species)],
    methylkit = lambda wildcards:os.path.join(config["directories"]["methylation_call"], f"{wildcards.sample}_"+f"{wildcards.species}"+".txt")
  threads:5
  shell:
    """
    enva run htseq -- htseq-count -f bam -r pos -s yes -t exon -i gene_id -m intersection-nonempty \
    {input.bam_sorted} {params.rnaseq_gtf} > {params.methylkit}

    """
   




    
