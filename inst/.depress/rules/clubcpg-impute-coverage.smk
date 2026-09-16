rule clubcpgimputecoverage:
  message:"Calculating clubcpg impute coverage before imputation ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+"_Filtered.bam"),
    filter_csv = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["coverage"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.filtered.csv")
  output:
    os.path.join(config["directories"]["clubcpg"]["impute"], "CompleteBins."+"{sample}_"+config["workflow"]["species"]["graft"]+".bam."+"{chr}.IMPUTED_filter.csv")
  params:
    output_dir = config["directories"]["clubcpg"]["impute"],
    chr = lambda wildcards:f"{wildcards.chr}",
    models = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["model"],f"{wildcards.sample}",f"{wildcards.chr}"),
    read1_5 = config["workflow"]["trim"]["read1_5"],
    read1_3 = config["workflow"]["trim"]["read1_3"],
    read2_5 = config["workflow"]["trim"]["read2_5"],
    read2_3 = config["workflow"]["trim"]["read2_3"],
    filter_output = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["impute"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.IMPUTED_filter.csv"),
    origin_output = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["impute"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.csv.IMPUTED.csv")
  threads:24
  shell:
    """
    mkdir {params.models}
    samtools index {input.bam_sorted}
    enva run xdxtools-extra -- clugcpg-impute-coverage -a {input.bam_sorted} \
    -c {input.filter_csv} \
    -m {params.models} \
    -o {params.output_dir} \
    -n {threads} -chr {params.chr} \
    --read1_5 {params.read1_5} \
    --read1_3 {params.read1_3} \
    --read2_5 {params.read2_5} \
    --read2_3 {params.read2_3}
    
    cat {params.origin_output} | awk -F "," '$2>=10 && $3>=2' > {params.filter_output}
    
    """
