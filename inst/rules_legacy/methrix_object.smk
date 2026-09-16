rule create_methrix_object :
  message:"Build beta matrix ..."
  input:
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}_nsort.bismark.cov.gz"),sample = config["metadata"]["sample_ids"])
  output:
    os.path.join(config["directories"]["methylation_call"], "methrixh5","assays.h5"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","se.rds"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","bsseq.RDS"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","CpG_coverage.xlsx")

  params:
    mcall_dir = config["directories"]["methylation_call"],
    methrix_dir = os.path.join(config["directories"]["methylation_call"], "methrixh5"),
    genome = config["reference"]["annotations"]["names"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])]
  threads:10
  shell:
    """
      enva run base Rscript R/build_methrix.R \
      --filein {params.mcall_dir} \
      --fileout {params.methrix_dir} \
      --cores {threads} \
      --Genome {params.genome}

    """
   




    
