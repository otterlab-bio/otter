rule clubcpgcoverage:
  message:"Calculating clubcpg coverage before imputation ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+"_Filtered.bam")
  output:
    os.path.join(config["directories"]["clubcpg"]["coverage"], "CompleteBins."+"{sample}_"+config["workflow"]["species"]["graft"]+".bam."+"{chr}.filtered.csv")
  params:
    output_dir = config["directories"]["clubcpg"]["coverage"],
    bin_size = 100,
    chr = lambda wildcards:f"{wildcards.chr}",
    read1_5 = config["workflow"]["trim"]["read1_5"],
    read1_3 = config["workflow"]["trim"]["read1_3"],
    read2_5 = config["workflow"]["trim"]["read2_5"],
    read2_3 = config["workflow"]["trim"]["read2_3"],
    filter_output = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["coverage"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.filtered.csv"),
    origin_output = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["coverage"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+"_Filtered.bam."+f"{wildcards.chr}.csv")
  threads:24
  shell:
    """
    samtools index {input.bam_sorted}
    enva run clubcpg -- clubcpg-coverage -a {input.bam_sorted} -o {params.output_dir} \
    --bin_size {params.bin_size} \
    -n {threads} -chr {params.chr} \
    --read1_5 {params.read1_5} \
    --read1_3 {params.read1_3} \
    --read2_5 {params.read2_5} \
    --read2_3 {params.read2_3}
    
    cat {params.origin_output} | awk -F "," '$2>=10 && $3>=2' > {params.filter_output}
    
    """
    

