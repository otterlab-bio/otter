rule build_expression_matrix :
  message:"Build expression matrix ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"],"Filtered_bams" , f"{wildcards.sample}_fixed_"+config["workflow"]["species"]["graft"]+"_Filtered.bam")
  output:
    os.path.join(config["directories"]["methylation_call"], "{sample}_"+config["workflow"]["species"]["graft"]+".txt")
  params:
    rnaseq_gtf = lambda wildcards:config["reference"]["rnaseq"]["gtf"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])],
    methylkit = lambda wildcards:os.path.join(config["directories"]["methylation_call"], f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".txt")
  threads:5
  shell:
    """
    enva run htseq \
      htseq-count -f bam -r pos -s yes -t exon -i gene_id \
      -m intersection-nonempty \
      {input.bam_sorted} {params.rnaseq_gtf} > {params.methylkit}

    """
   




    
