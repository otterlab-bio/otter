rule clubcpgimputecluster:
  message:"calculating clubcpg impute cluster ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+"_Filtered.bam"),
    impute_csv = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["impute"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.IMPUTED_filter.csv")
  output:
    os.path.join(config["directories"]["clubcpg"]["main"],"{sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+".bam."+"{chr}_cluster_results.csv")
  params:
    output_folder = config["directories"]["clubcpg"]["main"],
    bin_size = 100,
    cluster_member_minimum = 4,
    chr = lambda wildcards:f"{wildcards.chr}",
    model_folder = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["model"],f"{wildcards.sample}",f"{wildcards.chr}"),
    read1_5 = config["workflow"]["trim"]["read1_5"],
    read1_3 = config["workflow"]["trim"]["read1_3"],
    read2_5 = config["workflow"]["trim"]["read2_5"],
    read2_3 = config["workflow"]["trim"]["read2_3"],
    seq_deth = config["workflow"]["trim"]["seq_deth"]
  threads:24
  shell:
    """
    samtools index {input.bam_sorted}
    enva run xdxtools-extra -- clubcpg-impute-cluster -a {input.bam_sorted} \
    --bins {input.impute_csv} \
    -o {params.output_folder} \
    --bin_size {params.bin_size} \
    -m {params.cluster_member_minimum} \
    -r {params.seq_deth} \
    -n {threads} \
    --suffix {params.chr} \
    --read1_5 {params.read1_5} \
    --read1_3 {params.read1_3} \
    --read2_5 {params.read2_5} \
    --read2_3 {params.read2_3} \
    --models_A {params.model_folder}
    
    """
