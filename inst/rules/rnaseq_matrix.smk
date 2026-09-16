rule construct_expression_matrix :
  message:"Construct expression matrix ..."
  input:
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}" + "_" + config["workflow"]["species"]["graft"] + ".txt"), sample=config["metadata"]["sample_ids"])
  output:
    os.path.join(config["directories"]["beta_matrix"], "matrix_count.txt"),
    os.path.join(config["directories"]["beta_matrix"], "matrix_norm.txt")
  params:
    htseq_dir = config["directories"]["methylation_call"],
    output_dir = config["directories"]["beta_matrix"],
    postfix = "_" + config["workflow"]["species"]["graft"] + ".txt",
    species = config["workflow"]["species"]["expression"]
  threads:5
  shell:
    """
    seq2mat \
      --htseq_dir {params.htseq_dir} \
      --output_dir {params.output_dir} \
      --postfix {params.postfix} \
      --species {params.species}

    """
   
