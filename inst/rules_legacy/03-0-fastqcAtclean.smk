rule fastqcAtclean:
  message:"Running FastQC after Trim ..."
  input:
    R1 = os.path.join(config["output"]["trim_dir"], "{sample}"  + "_val_1.fq.gz"),
    R2 = os.path.join(config["output"]["trim_dir"], "{sample}" + "_val_2.fq.gz")
  output:
    R1_fastqc_zip = os.path.join(config["directories"]["qc"]["after"], "{sample}" + "_val_1_fastqc.zip"),
    R1_fastqc_html = os.path.join(config["directories"]["qc"]["after"], "{sample}" + "_val_1_fastqc.html"),
    R2_fastqc_zip = os.path.join(config["directories"]["qc"]["after"], "{sample}" + "_val_2_fastqc.zip"),
    R2_fastqc_html = os.path.join(config["directories"]["qc"]["after"], "{sample}" + "_val_2_fastqc.html")
  params:
    dir=config["directories"]["qc"]["after"]
  threads: 6
  shell:
    """
    enva run fastqc -- fastqc -o {params.dir} -t {threads} --extract {input.R1} {input.R2}
    
    """
