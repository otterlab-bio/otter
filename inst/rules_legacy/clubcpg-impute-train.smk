rule clubcpgimputetrain:
  message:"clubcpg impute training"
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+config["workflow"]["trim"]["fixed"]+config["workflow"]["species"]["graft"]+"_Filtered.bam"),
    filter_csv = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["coverage"], "CompleteBins."+f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam."+f"{wildcards.chr}.filtered.csv")
  output:
    os.path.join(config["directories"]["clubcpg"]["model"], "{sample}","{chr}","saved_model_5_cpgs.prelim")
  params:
    limit_sample = 1000,
    model_folder = lambda wildcards:os.path.join(config["directories"]["clubcpg"]["model"],f"{wildcards.sample}",f"{wildcards.chr}"),
    read1_5 = config["workflow"]["trim"]["read1_5"],
    read1_3 = config["workflow"]["trim"]["read1_3"],
    read2_5 = config["workflow"]["trim"]["read2_5"],
    read2_3 = config["workflow"]["trim"]["read2_3"]
  threads:20
  shell:
     """
    mkdir {params.model_folder}
    samtools index {input.bam_sorted}
    enva run clubcpg -- clubcpg-impute-train -a {input.bam_sorted} \
    -c {input.filter_csv} \
    -o {params.model_folder} \
    -n {threads} \
    -l {params.limit_sample} \
    --read1_5 {params.read1_5} \
    --read1_3 {params.read1_3} \
    --read2_5 {params.read2_5} \
    --read2_3 {params.read2_3}
    
    """

