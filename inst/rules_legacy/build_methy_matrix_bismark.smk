rule bismark_methylation_extractor :
  message:"Build beta matrix ..."
  input:
    bam_sorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_"+config["workflow"]["species"]["graft"]+".bam")
  output:
     os.path.join(config["directories"]["methylation_call"], "{sample}_nsort.bismark.cov.gz"),
     os.path.join(config["directories"]["bsmap"]["main"], "{sample}_nsort.bam")
  params:
    mem_size = "20G",
    mcall_dir = config["directories"]["methylation_call"],
    methrix_dir = os.path.join(config["directories"]["methylation_call"], "methrixh5"),
    genomeFile = config["reference"]["indices"]["genome"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])],
    bam_nsorted = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_nsort.bam"),
    bam_nsorted_repaired = lambda wildcards:os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_repaired_nsort.bam"),
    bam_readnames = lambda wildcards:os.path.join(config["directories"]["methylation_call"], f"{wildcards.sample}_readnames.txt")
  threads:20
  shell:
    """
      enva run otter-core -- samtools sort  -@ {threads} -n -o {params.bam_nsorted} {input.bam_sorted}
      
      # repair unpaired reads
      enva run otter-core -- samtools view {params.bam_nsorted}  | awk '{{print $1}}' | sort > {params.bam_readnames}
      enva run base Rscript R/repair_unpaired_reads.R  --readname {params.bam_readnames}
      if [ -e {params.bam_readnames} ]; then
             enva run picard -- picard FilterSamReads I={params.bam_nsorted} O={params.bam_nsorted_repaired} READ_LIST_FILE={params.bam_readnames} FILTER=excludeReadList
             rm {params.bam_nsorted}
             mv {params.bam_nsorted_repaired} {params.bam_nsorted}
        fi
      
      bismark_methylation_extractor --paired-end --gzip \
      --output_dir {params.mcall_dir} \
      --comprehensive --merge_non_CpG --bedGraph --multicore 8 \
      --buffer_size {params.mem_size} \
      {params.bam_nsorted}

    """
   




    
